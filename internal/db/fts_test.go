package db

import (
	"fmt"
	"strings"
	"testing"

	"treehole/internal/models"
)

func TestTokenizeForSearch(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"hello", "hello"},
		{"二食堂", "二 食 堂"},
		{"今天去二食堂吃饭!", "今 天 去 二 食 堂 吃 饭"},
		{"GPA3.5以上", "GPA3 5 以 上"},
		{"树洞gpa查询", "树 洞 gpa 查 询"},
		{"!!!", ""},
		{"🎉🎊新年快乐", "新 年 快 乐"},
		{"mixed 中英 text混合", "mixed 中 英 text 混 合"},
		{"café", "café"},
	}
	for _, tc := range cases {
		if got := TokenizeForSearch(tc.in); got != tc.want {
			t.Errorf("TokenizeForSearch(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func requireFTS(t *testing.T, d *Database) {
	t.Helper()
	if !d.FTSEnabled() {
		t.Skip("built without sqlite_fts5 tag; FTS path unavailable")
	}
}

func TestFTSSearchChineseRecall(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	requireFTS(t, db)

	posts := []models.Post{
		{Pid: 1, Text: "今天去二食堂吃饭，人很多", Type: "text", Timestamp: 1000},
		{Pid: 2, Text: "保研经验分享，希望对大家有帮助", Type: "text", Timestamp: 2000},
		{Pid: 3, Text: "图书馆四楼很安静", Type: "text", Timestamp: 3000},
		{Pid: 4, Text: "食堂二楼新开了麻辣香锅窗口", Type: "text", Timestamp: 4000},
		{Pid: 5, Text: "PKU treehole is fun", Type: "text", Timestamp: 5000},
	}
	if err := db.SaveCrawlResult(posts, nil); err != nil {
		t.Fatalf("SaveCrawlResult: %v", err)
	}

	cases := []struct {
		query string
		pids  []int32
	}{
		{"食堂", []int32{4, 1}},
		{"二食堂", []int32{1}},
		{"保研", []int32{2}},
		{"食堂 PKU", nil},
		{"#3 图书馆", []int32{3}},
	}
	for _, tc := range cases {
		got, err := db.SearchPostsCursor(tc.query, 0, 10, false)
		if err != nil {
			t.Fatalf("SearchPostsCursor(%q): %v", tc.query, err)
		}
		if len(got) != len(tc.pids) {
			t.Fatalf("query %q: got %d results (%v), want pids %v", tc.query, len(got), pids(got), tc.pids)
		}
		for i, p := range got {
			if p.Pid != tc.pids[i] {
				t.Fatalf("query %q: result[%d].pid = %d, want %d (order by pid DESC)", tc.query, i, p.Pid, tc.pids[i])
			}
		}
	}
}

func TestFTSUpdateAndDeleteSync(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	requireFTS(t, db)

	post := models.Post{Pid: 10, Text: "原始内容包含关键词芒果", Type: "text", Timestamp: 1000}
	if err := db.UpsertPosts([]models.Post{post}); err != nil {
		t.Fatalf("UpsertPosts: %v", err)
	}
	got, err := db.SearchPostsCursor("芒果", 0, 10, false)
	if err != nil || len(got) != 1 {
		t.Fatalf("search after insert: %v %v", got, err)
	}

	post.Text = "更新后的内容提到荔枝"
	if err := db.UpsertPosts([]models.Post{post}); err != nil {
		t.Fatalf("UpsertPosts update: %v", err)
	}
	got, err = db.SearchPostsCursor("芒果", 0, 10, false)
	if err != nil {
		t.Fatalf("search old term: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("old term 芒果 should be gone after update, got %v", pids(got))
	}
	got, err = db.SearchPostsCursor("荔枝", 0, 10, false)
	if err != nil || len(got) != 1 {
		t.Fatalf("new term 荔枝 not found: %v %v", got, err)
	}

	if err := db.db.Exec("DELETE FROM posts WHERE pid = 10").Error; err != nil {
		t.Fatalf("delete post: %v", err)
	}
	got, err = db.SearchPostsCursor("荔枝", 0, 10, false)
	if err != nil || len(got) != 0 {
		t.Fatalf("term should be gone after delete: %v %v", got, err)
	}
}

func TestFTSSearchOrderByField(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	requireFTS(t, db)

	posts := []models.Post{
		{Pid: 1, Text: "期末复习资料分享", Type: "text", Timestamp: 1000, Reply: 5},
		{Pid: 2, Text: "期末考试安排出来了", Type: "text", Timestamp: 2000, Reply: 99},
		{Pid: 3, Text: "无关帖子", Type: "text", Timestamp: 3000, Reply: 1},
	}
	if err := db.SaveCrawlResult(posts, nil); err != nil {
		t.Fatalf("SaveCrawlResult: %v", err)
	}
	got, err := db.SearchPostsOrderBy("期末", "reply", 0, 10)
	if err != nil {
		t.Fatalf("SearchPostsOrderBy: %v", err)
	}
	if len(got) != 2 || got[0].Pid != 2 || got[1].Pid != 1 {
		t.Fatalf("SearchPostsOrderBy reply desc = %v, want [2 1]", pids(got))
	}
	got2, err := db.SearchPostsOrderBy("期末", "reply", 99, 10)
	if err != nil || len(got2) != 1 || got2[0].Pid != 1 {
		t.Fatalf("cursor page = %v %v, want [1]", pids(got2), err)
	}
}

func TestFTSStatusCounts(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	requireFTS(t, db)

	posts := make([]models.Post, 0, 5)
	for i := 1; i <= 5; i++ {
		posts = append(posts, models.Post{Pid: int32(i), Text: fmt.Sprintf("测试帖子%d", i), Type: "text", Timestamp: int32(1000 + i)})
	}
	if err := db.SaveCrawlResult(posts, nil); err != nil {
		t.Fatalf("SaveCrawlResult: %v", err)
	}
	st, err := db.FTSStatus()
	if err != nil {
		t.Fatalf("FTSStatus: %v", err)
	}
	if st.Posts != 5 || st.IndexedRows != 5 {
		t.Fatalf("status = %+v, want posts=5 indexed=5", st)
	}

	if _, err := db.BackfillFTSSearchColumn(nil); err != nil {
		t.Fatalf("BackfillFTSSearchColumn: %v", err)
	}

	extra := models.Post{Pid: 6, Text: "", Type: "text", Timestamp: 9999}
	if err := db.SaveCrawlResult([]models.Post{extra}, nil); err != nil {
		t.Fatalf("SaveCrawlResult empty text: %v", err)
	}
	n, err := db.BackfillFTSSearchColumn(nil)
	if err != nil {
		t.Fatalf("Backfill with empty-text row: %v", err)
	}
	if n != 0 && n != 1 {
		t.Fatalf("backfill processed %d rows, want 0 or 1 (empty text row)", n)
	}
}

func TestLikeFallbackWhenFTSDisabled(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	db.ftsEnabled = false
	if err := db.SaveCrawlResult([]models.Post{
		{Pid: 1, Text: "二食堂很好吃", Type: "text", Timestamp: 1000},
	}, nil); err != nil {
		t.Fatalf("SaveCrawlResult: %v", err)
	}
	got, err := db.SearchPostsCursor("食堂", 0, 10, false)
	if err != nil || len(got) != 1 {
		t.Fatalf("LIKE fallback search failed: %v %v", got, err)
	}
	if !strings.Contains(got[0].Text, "二食堂") {
		t.Fatalf("unexpected result: %+v", got[0])
	}
}

func pids(posts []models.Post) []int32 {
	out := make([]int32, 0, len(posts))
	for _, p := range posts {
		out = append(out, p.Pid)
	}
	return out
}
