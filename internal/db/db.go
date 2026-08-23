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
	db         *gorm.DB
	dbType     string
	ftsEnabled bool
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
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
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

	database := &Database{db: db, dbType: cfg.Database.Type}
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
	}).CreateInBatches(posts, 100).Error
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
	}).CreateInBatches(comments, 100).Error
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
	err := d.db.Raw("SELECT pid, text, anonymous, type, extra, timestamp, reply, likenum, status, is_comment, is_protect, is_top, label, media_ids FROM posts WHERE pid = ?", pid).First(&post).Error
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
	var posts []models.Post
	order := "DESC"
	comparator := "<"
	if sortAsc {
		order = "ASC"
		comparator = ">"
	}

	if cursor != 0 {
		err := d.db.Raw("SELECT pid, text, anonymous, type, extra, timestamp, reply, likenum, status, is_comment, is_protect, is_top, label, media_ids FROM posts WHERE pid "+comparator+" ? ORDER BY pid "+order+" LIMIT ?", cursor, limit).Scan(&posts).Error
		return posts, err
	}
	err := d.db.Raw("SELECT pid, text, anonymous, type, extra, timestamp, reply, likenum, status, is_comment, is_protect, is_top, label, media_ids FROM posts ORDER BY pid "+order+" LIMIT ?", limit).Scan(&posts).Error
	return posts, err
}

func (d *Database) SearchPostsCursor(keyword string, cursor int, limit int, sortAsc bool) ([]models.Post, error) {
	var posts []models.Post
	order := "DESC"
	cmp := "<"
	if sortAsc {
		order = "ASC"
		cmp = ">"
	}
	search := parsePostSearchQuery(keyword)
	if mq, ok := d.ftsMatchExpr(search); ok {
		inner := fmt.Sprintf("SELECT rowid FROM %s WHERE posts_fts MATCH ?", ftsTableName)
		args := []interface{}{mq}
		if cursor != 0 {
			inner += fmt.Sprintf(" AND rowid %s ?", cmp)
			args = append(args, cursor)
		}
		inner += fmt.Sprintf(" ORDER BY rowid %s LIMIT ?", order)
		args = append(args, limit)
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
	var posts []models.Post
	orderCol := validateOrderField(field)

	query := d.db.Model(&models.Post{}).Order(orderCol + " DESC")
	if cursor != 0 {
		query = query.Where(orderCol+" < ?", cursor)
	}

	err := query.Limit(limit).Find(&posts).Error
	return posts, err
}

func (d *Database) SearchPostsOrderBy(keyword string, field string, cursor int, limit int) ([]models.Post, error) {
	var posts []models.Post
	orderCol := validateOrderField(field)

	search := parsePostSearchQuery(keyword)
	if mq, ok := d.ftsMatchExpr(search); ok {
		sql := fmt.Sprintf("SELECT %s FROM %s f JOIN posts p ON p.pid = f.rowid WHERE posts_fts MATCH ?", qualifiedPostColumns("p"), ftsTableName)
		args := []interface{}{mq}
		if cursor != 0 {
			sql += " AND p." + orderCol + " < ?"
			args = append(args, cursor)
		}
		sql += " ORDER BY p." + orderCol + " DESC LIMIT ?"
		args = append(args, limit)
		err := d.db.Raw(sql, args...).Scan(&posts).Error
		return posts, err
	}

	query := applyPostSearch(d.db.Model(&models.Post{}), keyword).Order(orderCol + " DESC")
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
