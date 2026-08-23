package db

import (
	"fmt"
	"log"
	"strings"
	"unicode"
	"unicode/utf8"

	"gorm.io/gorm"
	"treehole/internal/models"
)

const ftsTableName = "posts_fts"

const ftsBackfillBatch = 2000

const postListColumns = "pid, text, anonymous, type, extra, timestamp, reply, likenum, status, is_comment, is_protect, is_top, label, media_ids"

func qualifiedPostColumns(prefix string) string {
	parts := strings.Split(postListColumns, ", ")
	for i := range parts {
		parts[i] = prefix + "." + parts[i]
	}
	return strings.Join(parts, ", ")
}

func isCJKRunes(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Bopomofo, r) ||
		unicode.Is(unicode.Hangul, r)
}

func isWordRune(r rune) bool {
	if r < utf8.RuneSelf {
		return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func TokenizeForSearch(text string) string {
	var b strings.Builder
	b.Grow(len(text) + 16)
	sep := false
	word := make([]rune, 0, 32)
	flush := func() {
		if len(word) == 0 {
			return
		}
		if sep {
			b.WriteByte(' ')
		}
		for _, r := range word {
			b.WriteRune(r)
		}
		sep = true
		word = word[:0]
	}
	for _, r := range text {
		switch {
		case isCJKRunes(r):
			flush()
			if sep {
				b.WriteByte(' ')
			}
			b.WriteRune(r)
			sep = true
		case isWordRune(r):
			word = append(word, r)
		default:
			flush()
		}
	}
	flush()
	return b.String()
}

func searchTokens(text string) []string {
	return strings.Fields(TokenizeForSearch(text))
}

func buildFTSMatchQuery(keywords []string) (string, bool) {
	var phrases []string
	for _, kw := range keywords {
		tokens := searchTokens(kw)
		if len(tokens) == 0 {
			continue
		}
		for i, t := range tokens {
			tokens[i] = strings.ReplaceAll(t, `"`, `""`)
		}
		phrases = append(phrases, `"`+strings.Join(tokens, " ")+`"`)
	}
	if len(phrases) == 0 {
		return "", false
	}
	return strings.Join(phrases, " AND "), true
}

func (d *Database) ftsMatchExpr(search postSearchQuery) (string, bool) {
	if !d.ftsEnabled || len(search.keywords) == 0 {
		return "", false
	}
	return buildFTSMatchQuery(search.keywords)
}

func (d *Database) initFTS() error {
	d.ftsEnabled = false
	if d.dbType != "sqlite3" {
		return nil
	}
	create := fmt.Sprintf(
		"CREATE VIRTUAL TABLE IF NOT EXISTS %s USING fts5(text_search, content='posts', content_rowid='pid', tokenize='unicode61')",
		ftsTableName,
	)
	if err := d.db.Exec(create).Error; err != nil {
		log.Printf("[FTS] FTS5 不可用，全文搜索回退为 LIKE: %v", err)
		return nil
	}
	triggers := []string{
		fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS posts_fts_ai AFTER INSERT ON posts BEGIN
	INSERT INTO %s(rowid, text_search) VALUES (new.pid, COALESCE(new.text_search, ''));
END`, ftsTableName),
		fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS posts_fts_ad AFTER DELETE ON posts
WHEN old.text_search IS NOT NULL
BEGIN
	INSERT INTO %s(%s, rowid, text_search) VALUES ('delete', old.pid, old.text_search);
END`, ftsTableName, ftsTableName),
		fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS posts_fts_au_del AFTER UPDATE OF text_search ON posts
WHEN new.text_search IS NOT old.text_search AND old.text_search IS NOT NULL
BEGIN
	INSERT INTO %s(%s, rowid, text_search) VALUES ('delete', old.pid, old.text_search);
	INSERT INTO %s(rowid, text_search) VALUES (new.pid, COALESCE(new.text_search, ''));
END`, ftsTableName, ftsTableName, ftsTableName),
		fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS posts_fts_au_ins AFTER UPDATE OF text_search ON posts
WHEN new.text_search IS NOT old.text_search AND old.text_search IS NULL
BEGIN
	INSERT INTO %s(rowid, text_search) VALUES (new.pid, COALESCE(new.text_search, ''));
END`, ftsTableName),
	}
	for _, stmt := range triggers {
		if err := d.db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("create fts trigger: %w", err)
		}
	}
	d.ftsEnabled = true
	log.Printf("[FTS] FTS5 全文索引已启用")
	return nil
}

func (d *Database) FTSEnabled() bool {
	return d != nil && d.ftsEnabled
}

func (d *Database) BackfillFTSSearchColumn(progress func(done, total int)) (int64, error) {
	if d.dbType != "sqlite3" {
		return 0, fmt.Errorf("fts backfill only supports sqlite3")
	}
	var total int64
	if err := d.db.Model(&models.Post{}).Count(&total).Error; err != nil {
		return 0, err
	}
	type row struct {
		Pid  int32
		Text string
	}
	var done int64
	watermark := int64(0)
	for {
		var rows []row
		err := d.db.Raw(
			"SELECT pid, text FROM posts WHERE pid > ? AND (text_search IS NULL OR text_search = '') ORDER BY pid LIMIT ?",
			watermark, ftsBackfillBatch,
		).Scan(&rows).Error
		if err != nil {
			return done, err
		}
		if len(rows) == 0 {
			break
		}
		watermark = int64(rows[len(rows)-1].Pid)
		err = d.db.Transaction(func(tx *gorm.DB) error {
			for _, r := range rows {
				if err := tx.Exec(
					"UPDATE posts SET text_search = ? WHERE pid = ?",
					TokenizeForSearch(sanitizeNullBytes(r.Text)), r.Pid,
				).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return done, err
		}
		done += int64(len(rows))
		if progress != nil {
			progress(int(done), int(total))
		}
	}
	return done, nil
}

func (d *Database) RebuildFTSIndex() error {
	if !d.ftsEnabled {
		return fmt.Errorf("FTS5 not enabled")
	}
	return d.db.Exec(fmt.Sprintf("INSERT INTO %s(%s) VALUES('rebuild')", ftsTableName, ftsTableName)).Error
}

type FTSStatus struct {
	Enabled     bool
	Posts       int64
	IndexedRows int64
	IndexBytes  int64
}

func (d *Database) FTSStatus() (*FTSStatus, error) {
	st := &FTSStatus{Enabled: d.ftsEnabled, IndexBytes: -1}
	posts, err := d.GetPostCount()
	if err != nil {
		return nil, err
	}
	st.Posts = int64(posts)
	if !d.ftsEnabled {
		return st, nil
	}
	if err := d.db.Raw("SELECT COUNT(*) FROM posts WHERE text_search IS NOT NULL").Scan(&st.IndexedRows).Error; err != nil {
		return nil, err
	}
	var size int64
	if err := d.db.Raw(fmt.Sprintf("SELECT COALESCE(SUM(pgsize), -1) FROM dbstat('%s')", ftsTableName)).Scan(&size).Error; err == nil {
		st.IndexBytes = size
	}
	return st, nil
}
