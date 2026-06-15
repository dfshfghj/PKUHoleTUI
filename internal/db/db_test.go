package db

import (
	"os"
	"testing"

	"treehole/internal/config"
	"treehole/internal/models"
)

func setupTestDB(t *testing.T) (*Database, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "test_*.db")
	if err != nil {
		t.Fatalf("create temp db: %v", err)
	}
	tmpFile.Close()

	cfg := &config.Config{
		Username:  "test",
		Password:  "test",
		SecretKey: "test",
		Database: config.DatabaseConfig{
			Type:   "sqlite3",
			DBFile: tmpFile.Name(),
		},
	}

	database, err := NewDatabase(cfg)
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("NewDatabase: %v", err)
	}

	cleanup := func() {
		database.Close()
		os.Remove(tmpFile.Name())
	}

	return database, cleanup
}

func seedPosts(t *testing.T, db *Database, posts []models.Post) {
	t.Helper()
	if err := db.UpsertPosts(posts); err != nil {
		t.Fatalf("UpsertPosts: %v", err)
	}
}

func seedComments(t *testing.T, db *Database, comments []models.Comment) {
	t.Helper()
	if err := db.UpsertComments(comments); err != nil {
		t.Fatalf("UpsertComments: %v", err)
	}
}

func TestUpsertPostsAndQuery(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	posts := []models.Post{
		{Pid: 1, Text: "Hello World", Type: "text", Timestamp: 1000, Anonymous: true, Reply: 5, Likenum: 10, PraiseNum: 3},
		{Pid: 2, Text: "Second Post", Type: "text", Timestamp: 2000, Anonymous: false, Reply: 2, Likenum: 8, PraiseNum: 1},
	}
	seedPosts(t, db, posts)

	count, err := db.GetPostCount()
	if err != nil {
		t.Fatalf("GetPostCount: %v", err)
	}
	if count != 2 {
		t.Errorf("Post count = %d, want 2", count)
	}

	post, err := db.GetPostByPid(1)
	if err != nil {
		t.Fatalf("GetPostByPid(1): %v", err)
	}
	if post.Text != "Hello World" {
		t.Errorf("Post[1].Text = %s, want 'Hello World'", post.Text)
	}
	if !post.Anonymous {
		t.Error("Post[1].Anonymous should be true")
	}
	if post.Reply != 5 {
		t.Errorf("Post[1].Reply = %d, want 5", post.Reply)
	}
}

func TestUpsertPostsConflictUpdate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	posts := []models.Post{
		{Pid: 1, Text: "Original", Type: "text", Timestamp: 1000, Anonymous: true},
	}
	seedPosts(t, db, posts)

	posts = []models.Post{
		{Pid: 1, Text: "Updated", Type: "text", Timestamp: 2000, Anonymous: false},
	}
	if err := db.UpsertPosts(posts); err != nil {
		t.Fatalf("UpsertPosts update: %v", err)
	}

	post, err := db.GetPostByPid(1)
	if err != nil {
		t.Fatalf("GetPostByPid(1): %v", err)
	}
	if post.Text != "Updated" {
		t.Errorf("Post[1].Text = %s, want 'Updated'", post.Text)
	}
	if post.Anonymous {
		t.Error("Post[1].Anonymous should be false after update")
	}
}

func TestUpsertPostsNullByteSanitize(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	posts := []models.Post{
		{Pid: 1, Text: "Hello\x00World", Type: "text", Timestamp: 1000},
	}
	if err := db.UpsertPosts(posts); err != nil {
		t.Fatalf("UpsertPosts: %v", err)
	}

	post, err := db.GetPostByPid(1)
	if err != nil {
		t.Fatalf("GetPostByPid(1): %v", err)
	}
	if post.Text != "HelloWorld" {
		t.Errorf("Post[1].Text = %q, want 'HelloWorld' (null bytes removed)", post.Text)
	}
}

func TestUpsertPostsBatch(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	var posts []models.Post
	for i := 1; i <= 150; i++ {
		posts = append(posts, models.Post{
			Pid: int32(i), Text: "Post", Type: "text", Timestamp: int32(i * 1000),
		})
	}
	seedPosts(t, db, posts)

	count, err := db.GetPostCount()
	if err != nil {
		t.Fatalf("GetPostCount: %v", err)
	}
	if count != 150 {
		t.Errorf("Post count = %d, want 150", count)
	}
}

func TestUpsertComments(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	seedPosts(t, db, []models.Post{
		{Pid: 1, Text: "Test Post", Type: "text", Timestamp: 1000},
	})

	comments := []models.Comment{
		{Cid: 1, Pid: 1, Text: "Comment 1", Timestamp: 1100, NameTag: "user1"},
		{Cid: 2, Pid: 1, Text: "Comment 2", Timestamp: 1200, NameTag: "user2"},
	}
	seedComments(t, db, comments)

	count, err := db.GetCommentCount()
	if err != nil {
		t.Fatalf("GetCommentCount: %v", err)
	}
	if count != 2 {
		t.Errorf("Comment count = %d, want 2", count)
	}

	fetched, err := db.GetCommentsByPidCursor(1, 0, 100, true)
	if err != nil {
		t.Fatalf("GetCommentsByPidCursor: %v", err)
	}
	if len(fetched) != 2 {
		t.Errorf("Got %d comments, want 2", len(fetched))
	}
}

func TestGetCommentsByPidCursorPreloadsQuote(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	seedPosts(t, db, []models.Post{
		{Pid: 1, Text: "Test Post", Type: "text", Timestamp: 1000},
	})

	quoteID := int32(1)
	seedComments(t, db, []models.Comment{
		{Cid: 1, Pid: 1, Text: "Quoted comment", Timestamp: 1100, NameTag: "user1"},
		{Cid: 2, Pid: 1, Text: "Reply comment", Timestamp: 1200, NameTag: "user2", QuoteID: &quoteID},
	})

	fetched, err := db.GetCommentsByPidCursor(1, 0, 100, true)
	if err != nil {
		t.Fatalf("GetCommentsByPidCursor: %v", err)
	}
	if len(fetched) != 2 {
		t.Fatalf("GetCommentsByPidCursor got %d comments, want 2", len(fetched))
	}
	if fetched[1].Quote == nil {
		t.Fatal("GetCommentsByPidCursor should preload Quote")
	}
	if fetched[1].Quote.NameTag != "user1" {
		t.Errorf("Quote.NameTag = %q, want %q", fetched[1].Quote.NameTag, "user1")
	}
	if fetched[1].Quote.Text != "Quoted comment" {
		t.Errorf("Quote.Text = %q, want %q", fetched[1].Quote.Text, "Quoted comment")
	}
}

func TestGetCommentByCid(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	seedPosts(t, db, []models.Post{
		{Pid: 1, Text: "Test Post", Type: "text", Timestamp: 1000},
	})

	quoteID := int32(1)
	seedComments(t, db, []models.Comment{
		{Cid: 1, Pid: 1, Text: "Quoted comment", Timestamp: 1100, NameTag: "user1"},
		{Cid: 2, Pid: 1, Text: "Reply comment", Timestamp: 1200, NameTag: "user2", QuoteID: &quoteID},
	})

	comment, err := db.GetCommentByCid(2)
	if err != nil {
		t.Fatalf("GetCommentByCid: %v", err)
	}
	if comment.Cid != 2 {
		t.Fatalf("Cid = %d, want 2", comment.Cid)
	}
	if comment.Quote == nil {
		t.Fatal("GetCommentByCid should preload Quote")
	}
	if comment.Quote.Text != "Quoted comment" {
		t.Fatalf("Quote.Text = %q, want %q", comment.Quote.Text, "Quoted comment")
	}
}

func TestUpsertCommentsNullByteSanitize(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	comments := []models.Comment{
		{Cid: 1, Pid: 1, Text: "Hello\x00World", Timestamp: 1000},
	}
	if err := db.UpsertComments(comments); err != nil {
		t.Fatalf("UpsertComments: %v", err)
	}

	fetched, err := db.GetCommentsByPidCursor(1, 0, 100, true)
	if err != nil {
		t.Fatalf("GetCommentsByPidCursor: %v", err)
	}
	if len(fetched) != 1 {
		t.Fatalf("Got %d comments, want 1", len(fetched))
	}
	if fetched[0].Text != "HelloWorld" {
		t.Errorf("Comment.Text = %q, want 'HelloWorld'", fetched[0].Text)
	}
}

func TestGetPostByPidNotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := db.GetPostByPid(99999)
	if err == nil {
		t.Fatal("GetPostByPid(99999) expected error, got nil")
	}
}

func TestGetPostsCursorPagination(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	var posts []models.Post
	for i := 1; i <= 25; i++ {
		posts = append(posts, models.Post{
			Pid: int32(i), Text: "Post", Type: "text", Timestamp: int32(i * 1000),
		})
	}
	seedPosts(t, db, posts)

	firstPage, err := db.GetPostsCursor(0, 10, false)
	if err != nil {
		t.Fatalf("GetPostsCursor(0,10,false): %v", err)
	}
	if len(firstPage) != 10 {
		t.Errorf("First page has %d posts, want 10", len(firstPage))
	}
	if firstPage[0].Pid != 25 {
		t.Errorf("First post pid = %d, want 25 (DESC order)", firstPage[0].Pid)
	}

	secondCursor := int(firstPage[len(firstPage)-1].Pid)
	secondPage, err := db.GetPostsCursor(secondCursor, 10, false)
	if err != nil {
		t.Fatalf("GetPostsCursor(cursor,10,false): %v", err)
	}
	if len(secondPage) != 10 {
		t.Errorf("Second page has %d posts, want 10", len(secondPage))
	}

	thirdCursor := int(secondPage[len(secondPage)-1].Pid)
	thirdPage, err := db.GetPostsCursor(thirdCursor, 10, false)
	if err != nil {
		t.Fatalf("GetPostsCursor(cursor,10,false): %v", err)
	}
	if len(thirdPage) != 5 {
		t.Errorf("Third page has %d posts, want 5", len(thirdPage))
	}
}

func TestGetPostsCursor(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	var posts []models.Post
	for i := 1; i <= 20; i++ {
		posts = append(posts, models.Post{
			Pid: int32(i), Text: "Post", Type: "text", Timestamp: int32(i * 1000),
		})
	}
	seedPosts(t, db, posts)

	first, err := db.GetPostsCursor(0, 5, false)
	if err != nil {
		t.Fatalf("GetPostsCursor(0,5,false): %v", err)
	}
	if len(first) != 5 {
		t.Fatalf("Got %d posts, want 5", len(first))
	}
	if first[0].Pid != 20 {
		t.Errorf("First post pid = %d, want 20 (DESC)", first[0].Pid)
	}

	cursor := int(first[len(first)-1].Pid)
	second, err := db.GetPostsCursor(cursor, 5, false)
	if err != nil {
		t.Fatalf("GetPostsCursor(cursor,5,false): %v", err)
	}
	if len(second) != 5 {
		t.Fatalf("Second page has %d posts, want 5", len(second))
	}
	if second[0].Pid != 15 {
		t.Errorf("Second page first pid = %d, want 15", second[0].Pid)
	}
}

func TestGetPostsCursorAsc(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	var posts []models.Post
	for i := 1; i <= 10; i++ {
		posts = append(posts, models.Post{
			Pid: int32(i), Text: "Post", Type: "text", Timestamp: int32(i * 1000),
		})
	}
	seedPosts(t, db, posts)

	page, err := db.GetPostsCursor(0, 5, true)
	if err != nil {
		t.Fatalf("GetPostsCursor(0,5,true): %v", err)
	}
	if len(page) != 5 {
		t.Fatalf("Got %d posts, want 5", len(page))
	}
	if page[0].Pid != 1 {
		t.Errorf("First post pid = %d, want 1 (ASC)", page[0].Pid)
	}
}

func TestSearchPostsCursorBasic(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	posts := []models.Post{
		{Pid: 1, Text: "PKU campus life", Type: "text", Timestamp: 1000},
		{Pid: 2, Text: "Library is great", Type: "text", Timestamp: 2000},
		{Pid: 3, Text: "PKU food review", Type: "text", Timestamp: 3000},
	}
	seedPosts(t, db, posts)

	results, err := db.SearchPostsCursor("PKU", 0, 10, false)
	if err != nil {
		t.Fatalf("SearchPostsCursor: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("SearchResults = %d, want 2", len(results))
	}

	results, err = db.SearchPostsCursor("nonexistent", 0, 10, false)
	if err != nil {
		t.Fatalf("SearchPostsCursor: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("SearchResults for 'nonexistent' = %d, want 0", len(results))
	}
}

func TestSearchPostsCursor(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	var posts []models.Post
	for i := 1; i <= 15; i++ {
		posts = append(posts, models.Post{
			Pid: int32(i), Text: "PKU post number", Type: "text", Timestamp: int32(i * 1000),
		})
	}
	seedPosts(t, db, posts)

	first, err := db.SearchPostsCursor("PKU", 0, 5, false)
	if err != nil {
		t.Fatalf("SearchPostsCursor(0,5,false): %v", err)
	}
	if len(first) != 5 {
		t.Fatalf("Got %d posts, want 5", len(first))
	}

	cursor := int(first[len(first)-1].Pid)
	second, err := db.SearchPostsCursor("PKU", cursor, 5, false)
	if err != nil {
		t.Fatalf("SearchPostsCursor(cursor,5,false): %v", err)
	}
	if len(second) != 5 {
		t.Fatalf("Second page has %d posts, want 5", len(second))
	}
}

func TestSearchPostsCursorByPidAndMultiKeyword(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	posts := []models.Post{
		{Pid: 101, Text: "PKU food review", Type: "text", Timestamp: 1000},
		{Pid: 102, Text: "PKU campus food", Type: "text", Timestamp: 2000},
		{Pid: 103, Text: "campus only", Type: "text", Timestamp: 3000},
	}
	seedPosts(t, db, posts)

	results, err := db.SearchPostsCursor("PKU food", 0, 10, false)
	if err != nil {
		t.Fatalf("SearchPostsCursor multi keyword: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("multi keyword results = %d, want 2", len(results))
	}

	results, err = db.SearchPostsCursor("#102", 0, 10, false)
	if err != nil {
		t.Fatalf("SearchPostsCursor by pid: %v", err)
	}
	if len(results) != 1 || results[0].Pid != 102 {
		t.Fatalf("pid search returned %+v, want pid 102", results)
	}

	results, err = db.SearchPostsCursor("#101 review", 0, 10, false)
	if err != nil {
		t.Fatalf("SearchPostsCursor by pid + keyword: %v", err)
	}
	if len(results) != 1 || results[0].Pid != 101 {
		t.Fatalf("pid+keyword search returned %+v, want pid 101", results)
	}
}

func TestGetPostsOrderBy(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	posts := []models.Post{
		{Pid: 1, Text: "A", Type: "text", Timestamp: 1000, Reply: 10, Likenum: 5, PraiseNum: 20},
		{Pid: 2, Text: "B", Type: "text", Timestamp: 2000, Reply: 30, Likenum: 15, PraiseNum: 5},
		{Pid: 3, Text: "C", Type: "text", Timestamp: 3000, Reply: 20, Likenum: 25, PraiseNum: 10},
	}
	seedPosts(t, db, posts)

	byReply, err := db.GetPostsOrderBy("reply", 0, 10)
	if err != nil {
		t.Fatalf("GetPostsOrderBy(reply): %v", err)
	}
	if len(byReply) != 3 {
		t.Fatalf("Got %d posts, want 3", len(byReply))
	}
	if byReply[0].Reply != 30 {
		t.Errorf("Top by reply = %d, want 30", byReply[0].Reply)
	}

	byLike, err := db.GetPostsOrderBy("likenum", 0, 10)
	if err != nil {
		t.Fatalf("GetPostsOrderBy(likenum): %v", err)
	}
	if byLike[0].Likenum != 25 {
		t.Errorf("Top by likenum = %d, want 25", byLike[0].Likenum)
	}

	byPraise, err := db.GetPostsOrderBy("praise_num", 0, 10)
	if err != nil {
		t.Fatalf("GetPostsOrderBy(praise_num): %v", err)
	}
	if byPraise[0].PraiseNum != 20 {
		t.Errorf("Top by praise_num = %d, want 20", byPraise[0].PraiseNum)
	}

	invalidField, err := db.GetPostsOrderBy("invalid", 0, 10)
	if err != nil {
		t.Fatalf("GetPostsOrderBy(invalid): %v", err)
	}
	if len(invalidField) != 3 {
		t.Errorf("Invalid field should fallback to pid, got %d posts", len(invalidField))
	}
}

func TestSearchPostsOrderBy(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	posts := []models.Post{
		{Pid: 1, Text: "PKU test A", Type: "text", Timestamp: 1000, Reply: 10},
		{Pid: 2, Text: "PKU test B", Type: "text", Timestamp: 2000, Reply: 50},
		{Pid: 3, Text: "Other", Type: "text", Timestamp: 3000, Reply: 100},
	}
	seedPosts(t, db, posts)

	results, err := db.SearchPostsOrderBy("PKU", "reply", 0, 10)
	if err != nil {
		t.Fatalf("SearchPostsOrderBy: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Got %d results, want 2", len(results))
	}
	if results[0].Reply != 50 {
		t.Errorf("Top PKU by reply = %d, want 50", results[0].Reply)
	}
}

func TestGetPostsWithImages(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	posts := []models.Post{
		{Pid: 1, Text: "Text only", Type: "text", Timestamp: 1000},
		{Pid: 2, Text: "Image post", Type: "image", Timestamp: 2000, MediaIds: "1,2"},
		{Pid: 3, Text: "Has media", Type: "text", Timestamp: 3000, MediaIds: "3"},
	}
	seedPosts(t, db, posts)

	count, err := db.GetPostsWithImagesCount()
	if err != nil {
		t.Fatalf("GetPostsWithImagesCount: %v", err)
	}
	if count != 2 {
		t.Errorf("Image posts count = %d, want 2", count)
	}

	imagePosts, err := db.GetPostsWithImages(0, 10)
	if err != nil {
		t.Fatalf("GetPostsWithImages: %v", err)
	}
	if len(imagePosts) != 2 {
		t.Errorf("Got %d image posts, want 2", len(imagePosts))
	}
}

func TestGetCommentsWithImages(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	seedPosts(t, db, []models.Post{
		{Pid: 1, Text: "Post", Type: "text", Timestamp: 1000},
	})

	comments := []models.Comment{
		{Cid: 1, Pid: 1, Text: "No image", Timestamp: 1000},
		{Cid: 2, Pid: 1, Text: "Has image", Timestamp: 2000, MediaIds: "10"},
		{Cid: 3, Pid: 1, Text: "Has image too", Timestamp: 3000, MediaIds: "11,12"},
	}
	seedComments(t, db, comments)

	count, err := db.GetCommentsWithImagesCount()
	if err != nil {
		t.Fatalf("GetCommentsWithImagesCount: %v", err)
	}
	if count != 2 {
		t.Errorf("Image comments count = %d, want 2", count)
	}
}

func TestGetCommentsByPidCursor(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	seedPosts(t, db, []models.Post{
		{Pid: 1, Text: "Post", Type: "text", Timestamp: 1000},
	})

	var comments []models.Comment
	for i := 1; i <= 15; i++ {
		comments = append(comments, models.Comment{
			Cid: int32(i), Pid: 1, Text: "Comment", Timestamp: int32(1000 + i*100),
		})
	}
	seedComments(t, db, comments)

	desc, err := db.GetCommentsByPidCursor(1, 0, 5, false)
	if err != nil {
		t.Fatalf("GetCommentsByPidCursor DESC: %v", err)
	}
	if len(desc) != 5 {
		t.Fatalf("DESC got %d comments, want 5", len(desc))
	}
	if desc[0].Cid != 15 {
		t.Errorf("DESC first cid = %d, want 15", desc[0].Cid)
	}

	asc, err := db.GetCommentsByPidCursor(1, 0, 5, true)
	if err != nil {
		t.Fatalf("GetCommentsByPidCursor ASC: %v", err)
	}
	if len(asc) != 5 {
		t.Fatalf("ASC got %d comments, want 5", len(asc))
	}
	if asc[0].Cid != 1 {
		t.Errorf("ASC first cid = %d, want 1", asc[0].Cid)
	}

	cursor := int32(desc[len(desc)-1].Cid)
	nextPage, err := db.GetCommentsByPidCursor(1, cursor, 5, false)
	if err != nil {
		t.Fatalf("GetCommentsByPidCursor with cursor: %v", err)
	}
	if len(nextPage) != 5 {
		t.Fatalf("Next page got %d comments, want 5", len(nextPage))
	}
	if nextPage[0].Cid != 10 {
		t.Errorf("Next page first cid = %d, want 10", nextPage[0].Cid)
	}
}

func TestUpsertExclusiveIdInfo(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	info := models.ExclusiveIdInfo{
		Id: 1, ExclusiveId: "test123", ApplicationTime: "2024-01-01",
		IsV: 1, AuditStatus: "approved", AuditTime: "2024-01-02",
		IsDel: 0, CreatedAt: "2024-01-01", UpdatedAt: "2024-01-02",
	}
	if err := db.UpsertExclusiveIdInfo(info); err != nil {
		t.Fatalf("UpsertExclusiveIdInfo: %v", err)
	}

	var fetched models.ExclusiveIdInfo
	if err := db.db.First(&fetched, 1).Error; err != nil {
		t.Fatalf("Query ExclusiveIdInfo: %v", err)
	}
	if fetched.ExclusiveId != "test123" {
		t.Errorf("ExclusiveId = %s, want test123", fetched.ExclusiveId)
	}
}

func TestDatabaseCounts(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	postCount, err := db.GetPostCount()
	if err != nil {
		t.Fatalf("GetPostCount: %v", err)
	}
	if postCount != 0 {
		t.Errorf("Empty DB PostCount = %d, want 0", postCount)
	}

	commentCount, err := db.GetCommentCount()
	if err != nil {
		t.Fatalf("GetCommentCount: %v", err)
	}
	if commentCount != 0 {
		t.Errorf("Empty DB CommentCount = %d, want 0", commentCount)
	}
}
