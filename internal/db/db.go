package db

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
	"treehole/internal/config"
	"treehole/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// containsNullByte checks if a string contains null bytes (0x00)
func containsNullByte(s string) bool {
	return strings.ContainsRune(s, '\x00')
}

// sanitizeNullBytes removes null bytes from a string
func sanitizeNullBytes(s string) string {
	return strings.ReplaceAll(s, "\x00", "")
}

// escapeLikePattern escapes % and _ characters in a LIKE pattern to prevent wildcard injection
func escapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return "%" + s + "%"
}

type postSearchQuery struct {
	pid      *int32
	keywords []string
}

func parsePostSearchQuery(raw string) postSearchQuery {
	var q postSearchQuery
	for _, token := range strings.Fields(raw) {
		if strings.HasPrefix(token, "#") && len(token) > 1 {
			if pid, err := strconv.ParseInt(token[1:], 10, 32); err == nil {
				p := int32(pid)
				q.pid = &p
				continue
			}
		}
		q.keywords = append(q.keywords, token)
	}
	return q
}

func applyPostSearch(query *gorm.DB, raw string) *gorm.DB {
	search := parsePostSearchQuery(raw)
	if search.pid != nil {
		query = query.Where("pid = ?", *search.pid)
	}
	for _, keyword := range search.keywords {
		query = query.Where("text LIKE ?", escapeLikePattern(keyword))
	}
	return query
}

type Database struct {
	db              *gorm.DB
	dbType          string
	ftsEnabled      bool
	ftsNeedsRebuild bool
	counts          *ttlCountCache
}

// TimeRange 表示按 timestamp 过滤的闭区间（Unix 秒）；0 表示该侧不设限
type TimeRange struct {
	Begin int64
	End   int64
}

func (r *TimeRange) empty() bool {
	return r == nil || (r.Begin <= 0 && r.End <= 0)
}

// applyConds 把时间区间条件追加到 SQL 片段与参数中
func (r *TimeRange) applyConds(conds []string, args []interface{}, col string) ([]string, []interface{}) {
	if r.empty() {
		return conds, args
	}
	if r.Begin > 0 {
		conds = append(conds, col+" >= ?")
		args = append(args, r.Begin)
	}
	if r.End > 0 {
		conds = append(conds, col+" <= ?")
		args = append(args, r.End)
	}
	return conds, args
}

// applySQL 把时间区间条件以 AND 形式追加到已含 WHERE 的原始 SQL 与参数中
func (r *TimeRange) applySQL(sql string, args []interface{}, col string) (string, []interface{}) {
	if r.empty() {
		return sql, args
	}
	if r.Begin > 0 {
		sql += " AND " + col + " >= ?"
		args = append(args, r.Begin)
	}
	if r.End > 0 {
		sql += " AND " + col + " <= ?"
		args = append(args, r.End)
	}
	return sql, args
}

// applyQuery 把时间区间条件追加到 GORM 查询
func (r *TimeRange) applyQuery(query *gorm.DB) *gorm.DB {
	if r.empty() {
		return query
	}
	if r.Begin > 0 {
		query = query.Where("timestamp >= ?", r.Begin)
	}
	if r.End > 0 {
		query = query.Where("timestamp <= ?", r.End)
	}
	return query
}

func NewDatabase(cfg *config.Config) (*Database, error) {
	dsn, err := cfg.GetDatabaseDSN()
	if err != nil {
		return nil, err
	}

	var db *gorm.DB
	gormCfg := &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent),
		DisableForeignKeyConstraintWhenMigrating: true,
	}
	switch cfg.Database.Type {
	case "sqlite3":
		db, err = gorm.Open(sqlite.Open(dsn), gormCfg)
	case "postgres":
		db, err = gorm.Open(postgres.Open(dsn), gormCfg)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Database.Type)
	}

	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	if cfg.Database.Type == "sqlite3" {
		maxConns := cfg.Database.MaxConns
		if maxConns < 1 {
			maxConns = 1
		}
		sqlDB.SetMaxOpenConns(maxConns)
		sqlDB.SetMaxIdleConns(maxConns)
		if err := configureSQLite(db); err != nil {
			_ = sqlDB.Close()
			return nil, err
		}
	} else {
		sqlDB.SetMaxOpenConns(25)
		sqlDB.SetMaxIdleConns(25)
		sqlDB.SetConnMaxLifetime(time.Hour)
		sqlDB.SetConnMaxIdleTime(10 * time.Minute)
	}

	database := &Database{db: db, dbType: cfg.Database.Type, counts: newTTLCountCache()}
	err = database.initTables()
	if err != nil {
		return nil, err
	}

	return database, nil
}

func configureSQLite(db *gorm.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	}
	for _, pragma := range pragmas {
		if err := db.Exec(pragma).Error; err != nil {
			return fmt.Errorf("configure sqlite %s: %w", pragma, err)
		}
	}
	return nil
}

func (d *Database) initTables() error {
	if err := d.db.AutoMigrate(&models.Post{}, &models.Comment{}, &models.ExclusiveIdInfo{}); err != nil {
		return err
	}
	return d.initFTS()
}

func (d *Database) UpsertPosts(posts []models.Post) error {
	sanitizePosts(posts)
	err := upsertPosts(d.db, posts)

	if err != nil {
		pids := make([]int32, len(posts))
		for i, p := range posts {
			pids[i] = p.Pid
		}
		log.Printf("[Database] UpsertPosts 失败: %v | 涉及帖子 PIDs: %v", err, pids)
	}

	return err
}

func sanitizePosts(posts []models.Post) {
	for i := range posts {
		if containsNullByte(posts[i].Text) {
			log.Printf("[Database] Post pid=%d 包含 null 字节，已清理。Text前100字符: %q",
				posts[i].Pid, sanitizeNullBytes(posts[i].Text[:min(len(posts[i].Text), 100)]))
			posts[i].Text = sanitizeNullBytes(posts[i].Text)
		}
		if containsNullByte(posts[i].IdentityInfo) {
			log.Printf("[Database] Post pid=%d IdentityInfo 包含 null 字节，已清理", posts[i].Pid)
			posts[i].IdentityInfo = sanitizeNullBytes(posts[i].IdentityInfo)
		}
		if containsNullByte(posts[i].ExclusiveIdInfo) {
			log.Printf("[Database] Post pid=%d ExclusiveIdInfo 包含 null 字节，已清理", posts[i].Pid)
			posts[i].ExclusiveIdInfo = sanitizeNullBytes(posts[i].ExclusiveIdInfo)
		}
		if containsNullByte(posts[i].Mention) {
			log.Printf("[Database] Post pid=%d Mention 包含 null 字节，已清理", posts[i].Pid)
			posts[i].Mention = sanitizeNullBytes(posts[i].Mention)
		}
		if containsNullByte(posts[i].ImageSize) {
			posts[i].ImageSize = sanitizeNullBytes(posts[i].ImageSize)
		}

		// 检查嵌套的评论
		for j := range posts[i].Comments {
			if containsNullByte(posts[i].Comments[j].Text) {
				log.Printf("[Database] Post pid=%d 的评论 cid=%d 包含 null 字节，已清理。Text前100字符: %q",
					posts[i].Pid, posts[i].Comments[j].Cid, sanitizeNullBytes(posts[i].Comments[j].Text[:min(len(posts[i].Comments[j].Text), 100)]))
				posts[i].Comments[j].Text = sanitizeNullBytes(posts[i].Comments[j].Text)
			}
			if containsNullByte(posts[i].Comments[j].IdentityInfo) {
				log.Printf("[Database] Post pid=%d 的评论 cid=%d IdentityInfo 包含 null 字节，已清理", posts[i].Pid, posts[i].Comments[j].Cid)
				posts[i].Comments[j].IdentityInfo = sanitizeNullBytes(posts[i].Comments[j].IdentityInfo)
			}
			if containsNullByte(posts[i].Comments[j].ExclusiveIdInfo) {
				log.Printf("[Database] Post pid=%d 的评论 cid=%d ExclusiveIdInfo 包含 null 字节，已清理", posts[i].Pid, posts[i].Comments[j].Cid)
				posts[i].Comments[j].ExclusiveIdInfo = sanitizeNullBytes(posts[i].Comments[j].ExclusiveIdInfo)
			}
			if containsNullByte(posts[i].Comments[j].Mention) {
				posts[i].Comments[j].Mention = sanitizeNullBytes(posts[i].Comments[j].Mention)
			}
			if containsNullByte(posts[i].Comments[j].NameTag) {
				posts[i].Comments[j].NameTag = sanitizeNullBytes(posts[i].Comments[j].NameTag)
			}
		}
	}
}

func upsertPosts(tx *gorm.DB, posts []models.Post) error {
	if len(posts) == 0 {
		return nil
	}
	for i := range posts {
		posts[i].TextSearch = TokenizeForSearch(posts[i].Text)
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "pid"}},
		UpdateAll: true,
	}).Omit(clause.Associations).CreateInBatches(posts, 100).Error
}

func (d *Database) SaveCrawlResult(posts []models.Post, comments []models.Comment) error {
	sanitizePosts(posts)
	sanitizeComments(comments)
	return d.db.Transaction(func(tx *gorm.DB) error {
		if err := upsertPosts(tx, posts); err != nil {
			return err
		}
		return upsertComments(tx, comments)
	})
}

// SaveCrawlSnapshot saves complete comment snapshots for the supplied posts.
// Callers must finish every comment page before using this method; partial crawls
// must continue using SaveCrawlResult.
func (d *Database) SaveCrawlSnapshot(posts []models.Post, comments []models.Comment, missingPostPIDs []int32) error {
	sanitizePosts(posts)
	sanitizeComments(comments)
	for i := range posts {
		posts[i].DeletedAt = 0
	}
	for i := range comments {
		comments[i].DeletedAt = 0
	}
	deletedAt := time.Now().Unix()
	return d.db.Transaction(func(tx *gorm.DB) error {
		// Mark first, then restore observed comments with the upsert below. This
		// avoids an unbounded NOT IN list for posts with many comment pages.
		// Existing markers keep their first detection time.
		for _, post := range posts {
			if err := tx.Model(&models.Comment{}).Where("pid = ? AND deleted_at = 0", post.Pid).Update("deleted_at", deletedAt).Error; err != nil {
				return err
			}
		}
		for _, pid := range missingPostPIDs {
			if err := tx.Model(&models.Post{}).Where("pid = ? AND deleted_at = 0", pid).Update("deleted_at", deletedAt).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.Comment{}).Where("pid = ? AND deleted_at = 0", pid).Update("deleted_at", deletedAt).Error; err != nil {
				return err
			}
		}
		if err := upsertPosts(tx, posts); err != nil {
			return err
		}
		return upsertComments(tx, comments)
	})
}

// FindMissingPostPIDs compares a fully observed PID range against stored live
// posts. IDs that were never stored are deliberately excluded.
func (d *Database) FindMissingPostPIDs(minPID, maxPID int32, observed []int32) ([]int32, error) {
	var stored []int32
	if err := d.db.Model(&models.Post{}).Where("pid >= ? AND pid <= ? AND deleted_at = 0", minPID, maxPID).Order("pid").Pluck("pid", &stored).Error; err != nil {
		return nil, err
	}
	seen := make(map[int32]struct{}, len(observed))
	for _, pid := range observed {
		seen[pid] = struct{}{}
	}
	var missing []int32
	for _, pid := range stored {
		if _, ok := seen[pid]; !ok {
			missing = append(missing, pid)
		}
	}
	return missing, nil
}

func (d *Database) UpsertComments(comments []models.Comment) error {
	sanitizeComments(comments)
	err := upsertComments(d.db, comments)

	if err != nil {
		cids := make([]int32, len(comments))
		for i, c := range comments {
			cids[i] = c.Cid
		}
		log.Printf("[Database] UpsertComments 失败: %v | 涉及评论 CIDs: %v", err, cids)
	}

	return err
}

func sanitizeComments(comments []models.Comment) {
	for i := range comments {
		if containsNullByte(comments[i].Text) {
			log.Printf("[Database] Comment cid=%d pid=%d 包含 null 字节，已清理。Text前100字符: %q",
				comments[i].Cid, comments[i].Pid, sanitizeNullBytes(comments[i].Text[:min(len(comments[i].Text), 100)]))
			comments[i].Text = sanitizeNullBytes(comments[i].Text)
		}
		if containsNullByte(comments[i].IdentityInfo) {
			log.Printf("[Database] Comment cid=%d IdentityInfo 包含 null 字节，已清理", comments[i].Cid)
			comments[i].IdentityInfo = sanitizeNullBytes(comments[i].IdentityInfo)
		}
		if containsNullByte(comments[i].ExclusiveIdInfo) {
			log.Printf("[Database] Comment cid=%d ExclusiveIdInfo 包含 null 字节，已清理", comments[i].Cid)
			comments[i].ExclusiveIdInfo = sanitizeNullBytes(comments[i].ExclusiveIdInfo)
		}
		if containsNullByte(comments[i].Mention) {
			comments[i].Mention = sanitizeNullBytes(comments[i].Mention)
		}
		if containsNullByte(comments[i].NameTag) {
			comments[i].NameTag = sanitizeNullBytes(comments[i].NameTag)
		}
	}
}

func upsertComments(tx *gorm.DB, comments []models.Comment) error {
	if len(comments) == 0 {
		return nil
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "cid"}},
		UpdateAll: true,
	}).Omit(clause.Associations).CreateInBatches(comments, 100).Error
}

func (d *Database) UpsertExclusiveIdInfo(info models.ExclusiveIdInfo) error {
	return d.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(&info).Error
}

func (d *Database) Close() error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Checkpoint 执行WAL checkpoint
func (d *Database) Checkpoint() error {
	if d.dbType != "sqlite3" {
		return nil
	}
	return d.db.Exec("PRAGMA wal_checkpoint(RESTART)").Error
}

func (d *Database) GetPostCount() (int, error) {
	var count int64
	err := d.db.Model(&models.Post{}).Count(&count).Error
	return int(count), err
}

func (d *Database) GetCommentCount() (int, error) {
	var count int64
	err := d.db.Model(&models.Comment{}).Count(&count).Error
	return int(count), err
}

func (d *Database) GetPostByPid(pid int32) (*models.Post, error) {
	var post models.Post
	err := d.db.Raw("SELECT "+postListColumns+" FROM posts WHERE pid = ?", pid).First(&post).Error
	if err != nil {
		return nil, err
	}
	return &post, nil
}

func (d *Database) GetCommentByCid(cid int32) (*models.Comment, error) {
	var comment models.Comment
	err := d.db.Model(&models.Comment{}).Preload("Quote").First(&comment, "cid = ?", cid).Error
	if err != nil {
		return nil, err
	}
	return &comment, nil
}

// GetPostsCursor 游标分页获取帖子列表 (DESC)
func (d *Database) GetPostsCursor(cursor int, limit int, sortAsc bool) ([]models.Post, error) {
	return d.GetPostsCursorRange(nil, cursor, limit, sortAsc)
}

// GetPostsCursorRange 同上，可按 timestamp 闭区间过滤
func (d *Database) GetPostsCursorRange(tr *TimeRange, cursor int, limit int, sortAsc bool) ([]models.Post, error) {
	var posts []models.Post
	order := "DESC"
	comparator := "<"
	if sortAsc {
		order = "ASC"
		comparator = ">"
	}

	conds := make([]string, 0, 3)
	args := make([]interface{}, 0, 3)
	if cursor != 0 {
		conds = append(conds, "pid "+comparator+" ?")
		args = append(args, cursor)
	}
	conds, args = tr.applyConds(conds, args, "timestamp")
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ") + " "
	}
	sql := "SELECT " + postListColumns + " FROM posts " + where + "ORDER BY pid " + order + " LIMIT ?"
	args = append(args, limit)
	err := d.db.Raw(sql, args...).Scan(&posts).Error
	return posts, err
}

func (d *Database) SearchPostsCursor(keyword string, cursor int, limit int, sortAsc bool) ([]models.Post, error) {
	return d.SearchPostsCursorRange(keyword, nil, cursor, limit, sortAsc)
}

// SearchPostsCursorRange 关键词搜索 + 游标分页，可按 timestamp 闭区间过滤
func (d *Database) SearchPostsCursorRange(keyword string, tr *TimeRange, cursor int, limit int, sortAsc bool) ([]models.Post, error) {
	var posts []models.Post
	order := "DESC"
	cmp := "<"
	if sortAsc {
		order = "ASC"
		cmp = ">"
	}
	if q, ok := d.buildFTSQuery(keyword); ok {
		args := []interface{}{q.match}
		var inner string
		if tr.empty() {
			// 快路径：LIMIT 下推到 FTS 内层，无需回表即可截断
			inner = fmt.Sprintf("SELECT rowid FROM %s WHERE posts_fts MATCH ?", ftsTableName)
			if q.pid != nil {
				inner += " AND rowid = ?"
				args = append(args, *q.pid)
			}
			if cursor != 0 {
				inner += fmt.Sprintf(" AND rowid %s ?", cmp)
				args = append(args, cursor)
			}
			inner += fmt.Sprintf(" ORDER BY rowid %s LIMIT ?", order)
			args = append(args, limit)
		} else {
			// 带时间过滤时必须先过滤再 LIMIT：内层 JOIN 回表后应用时间条件
			inner = fmt.Sprintf(
				"SELECT f.rowid FROM %s f JOIN posts p ON p.pid = f.rowid WHERE posts_fts MATCH ?",
				ftsTableName,
			)
			if q.pid != nil {
				inner += " AND f.rowid = ?"
				args = append(args, *q.pid)
			}
			if cursor != 0 {
				inner += fmt.Sprintf(" AND f.rowid %s ?", cmp)
				args = append(args, cursor)
			}
			inner, args = tr.applySQL(inner, args, "p.timestamp")
			inner += fmt.Sprintf(" ORDER BY f.rowid %s LIMIT ?", order)
			args = append(args, limit)
		}
		sql := fmt.Sprintf(
			"SELECT %s FROM posts p WHERE p.pid IN (%s) ORDER BY p.pid %s",
			qualifiedPostColumns("p"), inner, order,
		)
		err := d.db.Raw(sql, args...).Scan(&posts).Error
		return posts, err
	}
	query := d.db.Model(&models.Post{}).
		Select(postListColumns)
	query = applyPostSearch(query, keyword)

	query = tr.applyQuery(query)
	if cursor != 0 {
		query = query.Where(fmt.Sprintf("pid %s ?", cmp), cursor)
	}
	err := query.Order("pid " + order).Limit(limit).Find(&posts).Error
	return posts, err
}

// GetCommentsByPidCursor 游标分页获取评论列表
func (d *Database) GetCommentsByPidCursor(pid int32, cursor int32, limit int, sortAsc bool) ([]models.Comment, error) {
	var comments []models.Comment
	query := d.db.Model(&models.Comment{}).Preload("Quote").Where("pid = ?", pid)

	if sortAsc {
		query = query.Order("cid ASC")
		if cursor != 0 {
			query = query.Where("cid > ?", cursor)
		}
	} else {
		query = query.Order("cid DESC")
		if cursor != 0 {
			query = query.Where("cid < ?", cursor)
		}
	}

	err := query.Limit(limit).Find(&comments).Error
	return comments, err
}

func (d *Database) GetPostsOrderBy(field string, cursor int, limit int) ([]models.Post, error) {
	return d.GetPostsOrderByRange(field, nil, cursor, limit)
}

// GetPostsOrderByRange 按指定字段排序获取帖子，可按 timestamp 闭区间过滤
func (d *Database) GetPostsOrderByRange(field string, tr *TimeRange, cursor int, limit int) ([]models.Post, error) {
	var posts []models.Post
	orderCol := validateOrderField(field)

	query := tr.applyQuery(d.db.Model(&models.Post{})).Order(orderCol + " DESC")
	if cursor != 0 {
		query = query.Where(orderCol+" < ?", cursor)
	}

	err := query.Limit(limit).Find(&posts).Error
	return posts, err
}

func (d *Database) SearchPostsOrderBy(keyword string, field string, cursor int, limit int) ([]models.Post, error) {
	return d.SearchPostsOrderByRange(keyword, field, nil, cursor, limit)
}

// SearchPostsOrderByRange 关键词搜索 + 指定字段排序，可按 timestamp 闭区间过滤
func (d *Database) SearchPostsOrderByRange(keyword string, field string, tr *TimeRange, cursor int, limit int) ([]models.Post, error) {
	var posts []models.Post
	orderCol := validateOrderField(field)

	if q, ok := d.buildFTSQuery(keyword); ok {
		sql := fmt.Sprintf(
			"SELECT %s FROM %s f JOIN posts p ON p.pid = f.rowid WHERE posts_fts MATCH ?",
			qualifiedPostColumns("p"), ftsTableName,
		)
		args := []interface{}{q.match}
		if q.pid != nil {
			sql += " AND p.pid = ?"
			args = append(args, *q.pid)
		}
		sql, args = tr.applySQL(sql, args, "p.timestamp")
		if cursor != 0 {
			sql += " AND p." + orderCol + " < ?"
			args = append(args, cursor)
		}
		sql += " ORDER BY p." + orderCol + " DESC LIMIT ?"
		args = append(args, limit)
		err := d.db.Raw(sql, args...).Scan(&posts).Error
		return posts, err
	}

	query := tr.applyQuery(applyPostSearch(d.db.Model(&models.Post{}), keyword)).Order(orderCol + " DESC")
	if cursor != 0 {
		query = query.Where(orderCol+" < ?", cursor)
	}

	err := query.Limit(limit).Find(&posts).Error
	return posts, err
}

// validateOrderField returns a safe column name from user input
func validateOrderField(field string) string {
	switch field {
	case "reply", "likenum", "praise_num":
		return field
	default:
		return "pid"
	}
}

// GetPostsWithImages 获取有图片的帖子（type=image 或 media_ids 不为空）
func (d *Database) GetPostsWithImages(offset, limit int) ([]models.Post, error) {
	var posts []models.Post
	err := d.db.Model(&models.Post{}).
		Select("pid, type, media_ids").
		Where("type = ? OR media_ids != ?", "image", "").
		Order("pid DESC").
		Offset(offset).Limit(limit).Find(&posts).Error
	return posts, err
}

func (d *Database) GetPostsWithImagesCount() (int, error) {
	var count int64
	err := d.db.Model(&models.Post{}).Where("type = ? OR media_ids != ?", "image", "").Count(&count).Error
	return int(count), err
}

// GetCommentsWithImages 获取有图片的评论（media_ids 不为空）
func (d *Database) GetCommentsWithImages(offset, limit int) ([]models.Comment, error) {
	var comments []models.Comment
	err := d.db.Model(&models.Comment{}).
		Select("cid, pid, media_ids").
		Where("media_ids != ?", "").
		Order("cid DESC").
		Offset(offset).Limit(limit).Find(&comments).Error
	return comments, err
}

func (d *Database) GetCommentsWithImagesCount() (int, error) {
	var count int64
	err := d.db.Model(&models.Comment{}).Where("media_ids != ?", "").Count(&count).Error
	return int(count), err
}
