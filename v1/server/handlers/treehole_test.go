package handles

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"treehole/internal/config"
	"treehole/internal/db"
	"treehole/internal/models"

	"github.com/gin-gonic/gin"
)

func setupTestDB(t *testing.T) (*db.Database, func()) {
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

	database, err := db.NewDatabase(cfg)
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

func setupTestRouter(t *testing.T, database *db.Database) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", Health)
	r.GET("/help", Help)
	r.GET("/posts", GetPosts(database))
	r.GET("/post/:pid", GetPost(database))
	r.GET("/comment", GetComment(database))
	r.GET("/comments/:pid", GetComments(database))
	r.GET("/media/image", GetImage)
	return r
}

func seedTestPosts(t *testing.T, database *db.Database, posts []models.Post) {
	t.Helper()
	if err := database.UpsertPosts(posts); err != nil {
		t.Fatalf("UpsertPosts: %v", err)
	}
}

func seedTestComments(t *testing.T, database *db.Database, comments []models.Comment) {
	t.Helper()
	if err := database.UpsertComments(comments); err != nil {
		t.Fatalf("UpsertComments: %v", err)
	}
}

func TestHealth(t *testing.T) {
	r := gin.New()
	r.GET("/health", Health)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want 200", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != float64(200) {
		t.Errorf("body.status = %v, want 200", body["status"])
	}
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatal("body.data is not a map")
	}
	if data["message"] != "PKU Hole API is running" {
		t.Errorf("body.data.message = %v, want 'PKU Hole API is running'", data["message"])
	}
}

func TestHelp(t *testing.T) {
	r := gin.New()
	r.GET("/help", Help)

	req := httptest.NewRequest("GET", "/help", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	data := body["data"].([]interface{})
	if len(data) < 6 {
		t.Fatalf("help entries = %d, want at least 6", len(data))
	}

	foundComment := false
	for _, item := range data {
		entry := item.(map[string]interface{})
		if entry["route"] == "/comment?cid=123" {
			foundComment = true
			if entry["description"] == "" {
				t.Fatal("comment route description should not be empty")
			}
		}
	}
	if !foundComment {
		t.Fatal("help output missing /comment route")
	}
}

func TestGetPostValid(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	r := setupTestRouter(t, database)

	seedTestPosts(t, database, []models.Post{
		{Pid: 1, Text: "Hello World", Type: "text", Timestamp: 1000, Anonymous: true, Reply: 5, Likenum: 10, PraiseNum: 3, MediaIds: "1"},
	})

	req := httptest.NewRequest("GET", "/post/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	data := body["data"].(map[string]interface{})
	if data["id"] != float64(1) {
		t.Errorf("data.id = %v, want 1", data["id"])
	}
	if data["text"] != "Hello World" {
		t.Errorf("data.text = %v, want 'Hello World'", data["text"])
	}
	if data["username"] != "anonymous" {
		t.Errorf("data.username = %v, want 'anonymous'", data["username"])
	}
	if data["media_ids"] != "1" {
		t.Errorf("data.media_ids = %v, want '1'", data["media_ids"])
	}
}

func TestGetPostNonAnonymous(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	r := setupTestRouter(t, database)

	seedTestPosts(t, database, []models.Post{
		{Pid: 1, Text: "Real name post", Type: "text", Timestamp: 1000, Anonymous: false},
	})

	req := httptest.NewRequest("GET", "/post/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	data := body["data"].(map[string]interface{})
	if data["username"] != "" {
		t.Errorf("data.username = %v, want empty string for non-anonymous", data["username"])
	}
}

func TestGetPostInvalid(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	r := setupTestRouter(t, database)

	req := httptest.NewRequest("GET", "/post/abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want 400", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["errid"] != "InvalidParam" {
		t.Errorf("errid = %v, want 'InvalidParam'", body["errid"])
	}
}

func TestGetPostNotFound(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	r := setupTestRouter(t, database)

	req := httptest.NewRequest("GET", "/post/99999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want 404", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["errid"] != "NotFound" {
		t.Errorf("errid = %v, want 'NotFound'", body["errid"])
	}
}

func TestGetPostsDefault(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	r := setupTestRouter(t, database)

	var posts []models.Post
	for i := 1; i <= 30; i++ {
		posts = append(posts, models.Post{
			Pid: int32(i), Text: "Post", Type: "text", Timestamp: int32(i * 1000),
		})
	}
	seedTestPosts(t, database, posts)

	req := httptest.NewRequest("GET", "/posts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	data := body["data"].([]interface{})
	if len(data) != 25 {
		t.Errorf("Default limit returned %d posts, want 25", len(data))
	}
}

func TestGetPostsWithKeyword(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	r := setupTestRouter(t, database)

	seedTestPosts(t, database, []models.Post{
		{Pid: 1, Text: "PKU campus life", Type: "text", Timestamp: 1000},
		{Pid: 2, Text: "Library is great", Type: "text", Timestamp: 2000},
		{Pid: 3, Text: "PKU food review", Type: "text", Timestamp: 3000},
	})

	req := httptest.NewRequest("GET", "/posts?keyword=PKU", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	data := body["data"].([]interface{})
	if len(data) != 2 {
		t.Errorf("Search 'PKU' returned %d posts, want 2", len(data))
	}
}

func TestGetPostsWithOrderBy(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	r := setupTestRouter(t, database)

	seedTestPosts(t, database, []models.Post{
		{Pid: 1, Text: "A", Type: "text", Timestamp: 1000, Reply: 10, Likenum: 5, PraiseNum: 20},
		{Pid: 2, Text: "B", Type: "text", Timestamp: 2000, Reply: 30, Likenum: 15, PraiseNum: 5},
		{Pid: 3, Text: "C", Type: "text", Timestamp: 3000, Reply: 20, Likenum: 25, PraiseNum: 10},
	})

	req := httptest.NewRequest("GET", "/posts?order_by=reply", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	data := body["data"].([]interface{})
	first := data[0].(map[string]interface{})
	if first["id"] != float64(2) {
		t.Errorf("Top by reply id = %v, want 2", first["id"])
	}
}

func TestGetPostsWithLimit(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	r := setupTestRouter(t, database)

	var posts []models.Post
	for i := 1; i <= 50; i++ {
		posts = append(posts, models.Post{
			Pid: int32(i), Text: "Post", Type: "text", Timestamp: int32(i * 1000),
		})
	}
	seedTestPosts(t, database, posts)

	req := httptest.NewRequest("GET", "/posts?limit=5", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	data := body["data"].([]interface{})
	if len(data) != 5 {
		t.Errorf("limit=5 returned %d posts, want 5", len(data))
	}
}

func TestGetPostsWithLimitClamp(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	r := setupTestRouter(t, database)

	var posts []models.Post
	for i := 1; i <= 200; i++ {
		posts = append(posts, models.Post{
			Pid: int32(i), Text: "Post", Type: "text", Timestamp: int32(i * 1000),
		})
	}
	seedTestPosts(t, database, posts)

	req := httptest.NewRequest("GET", "/posts?limit=999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	data := body["data"].([]interface{})
	if len(data) != 100 {
		t.Errorf("limit=999 clamped to %d, want 100", len(data))
	}
}

func TestGetPostsWithBegin(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	r := setupTestRouter(t, database)

	var posts []models.Post
	for i := 1; i <= 30; i++ {
		posts = append(posts, models.Post{
			Pid: int32(i), Text: "Post", Type: "text", Timestamp: int32(i * 1000),
		})
	}
	seedTestPosts(t, database, posts)

	req := httptest.NewRequest("GET", "/posts?begin=26", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	data := body["data"].([]interface{})
	if len(data) != 25 {
		t.Errorf("begin=26 returned %d posts, want 25 (default limit)", len(data))
	}
}

func TestGetPostsWithInvalidLimit(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	r := setupTestRouter(t, database)

	seedTestPosts(t, database, []models.Post{
		{Pid: 1, Text: "Post", Type: "text", Timestamp: 1000},
	})

	req := httptest.NewRequest("GET", "/posts?limit=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	data := body["data"].([]interface{})
	if len(data) != 1 {
		t.Errorf("Invalid limit should default to 25, got %d posts", len(data))
	}
}

func TestGetComments(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	r := setupTestRouter(t, database)

	seedTestPosts(t, database, []models.Post{
		{Pid: 1, Text: "Post", Type: "text", Timestamp: 1000},
	})

	seedTestComments(t, database, []models.Comment{
		{Cid: 1, Pid: 1, Text: "Comment 1", Timestamp: 1100, NameTag: "user1"},
		{Cid: 2, Pid: 1, Text: "Comment 2", Timestamp: 1200, NameTag: "user2"},
		{Cid: 3, Pid: 1, Text: "Comment 3", Timestamp: 1300, NameTag: "user3"},
	})

	req := httptest.NewRequest("GET", "/comments/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	data := body["data"].([]interface{})
	if len(data) != 3 {
		t.Errorf("Got %d comments, want 3", len(data))
	}

	first := data[0].(map[string]interface{})
	if first["text"] != "Comment 1" {
		t.Errorf("First comment text = %v, want 'Comment 1'", first["text"])
	}
}

func TestGetCommentByCid(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	r := setupTestRouter(t, database)

	seedTestPosts(t, database, []models.Post{
		{Pid: 1, Text: "Post", Type: "text", Timestamp: 1000},
	})

	quoteID := int32(1)
	seedTestComments(t, database, []models.Comment{
		{Cid: 1, Pid: 1, Text: "Quoted", Timestamp: 1100, NameTag: "user1"},
		{Cid: 2, Pid: 1, Text: "Reply", Timestamp: 1200, NameTag: "user2", QuoteID: &quoteID, MediaIds: "2"},
	})

	req := httptest.NewRequest("GET", "/comment?cid=2", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	data := body["data"].(map[string]interface{})
	if data["cid"] != float64(2) {
		t.Fatalf("cid = %v, want 2", data["cid"])
	}
	if data["text"] != "Reply" {
		t.Fatalf("text = %v, want Reply", data["text"])
	}
	if data["media_ids"] != "2" {
		t.Fatalf("media_ids = %v, want 2", data["media_ids"])
	}
	if data["quote"] == nil {
		t.Fatal("quote should be preloaded")
	}
}

func TestGetCommentInvalidCid(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	r := setupTestRouter(t, database)

	req := httptest.NewRequest("GET", "/comment?cid=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Status = %d, want 400", w.Code)
	}
}

func TestGetCommentNotFound(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	r := setupTestRouter(t, database)

	req := httptest.NewRequest("GET", "/comment?cid=9999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("Status = %d, want 404", w.Code)
	}
}

func TestGetCommentsWithSort(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	r := setupTestRouter(t, database)

	seedTestPosts(t, database, []models.Post{
		{Pid: 1, Text: "Post", Type: "text", Timestamp: 1000},
	})

	seedTestComments(t, database, []models.Comment{
		{Cid: 1, Pid: 1, Text: "First", Timestamp: 1100},
		{Cid: 2, Pid: 1, Text: "Second", Timestamp: 1200},
		{Cid: 3, Pid: 1, Text: "Third", Timestamp: 1300},
	})

	req := httptest.NewRequest("GET", "/comments/1?sort=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	data := body["data"].([]interface{})
	first := data[0].(map[string]interface{})
	if first["text"] != "Third" {
		t.Errorf("sort=1 (DESC) first comment = %v, want 'Third'", first["text"])
	}
}

func TestGetCommentsInvalidPid(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	r := setupTestRouter(t, database)

	req := httptest.NewRequest("GET", "/comments/abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want 400", w.Code)
	}
}

func TestGetCommentsEmpty(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	r := setupTestRouter(t, database)

	seedTestPosts(t, database, []models.Post{
		{Pid: 1, Text: "Post", Type: "text", Timestamp: 1000},
	})

	req := httptest.NewRequest("GET", "/comments/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	data := body["data"].([]interface{})
	if len(data) != 0 {
		t.Errorf("Empty comments returned %d, want 0", len(data))
	}
}

func TestGetImageNoParam(t *testing.T) {
	r := gin.New()
	r.GET("/media/image", GetImage)

	req := httptest.NewRequest("GET", "/media/image", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want 400", w.Code)
	}
}

func TestGetImageInvalidId(t *testing.T) {
	r := gin.New()
	r.GET("/media/image", GetImage)

	req := httptest.NewRequest("GET", "/media/image?id=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want 400", w.Code)
	}
}

func TestGetImageNotFound(t *testing.T) {
	r := gin.New()
	r.GET("/media/image", GetImage)

	req := httptest.NewRequest("GET", "/media/image?id=99999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want 404", w.Code)
	}
}

func TestGetImageByPid(t *testing.T) {
	r := gin.New()
	r.GET("/media/image", GetImage)

	req := httptest.NewRequest("GET", "/media/image?pid=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want 400", w.Code)
	}
}

func TestGetPostsWithOrderByAndKeyword(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	r := setupTestRouter(t, database)

	seedTestPosts(t, database, []models.Post{
		{Pid: 1, Text: "PKU test A", Type: "text", Timestamp: 1000, Reply: 10},
		{Pid: 2, Text: "PKU test B", Type: "text", Timestamp: 2000, Reply: 50},
		{Pid: 3, Text: "Other post", Type: "text", Timestamp: 3000, Reply: 100},
	})

	req := httptest.NewRequest("GET", "/posts?keyword=PKU&order_by=reply", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	data := body["data"].([]interface{})
	if len(data) != 2 {
		t.Errorf("Got %d results, want 2", len(data))
	}
	first := data[0].(map[string]interface{})
	if first["id"] != float64(2) {
		t.Errorf("Top PKU by reply id = %v, want 2", first["id"])
	}
}
