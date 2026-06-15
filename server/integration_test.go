package server

import (
	"net/http/httptest"
	"os"
	"testing"

	"treehole/internal/config"
	"treehole/internal/db"
	"treehole/internal/models"

	"github.com/gin-gonic/gin"
)

func setupTestEnv(t *testing.T) (*db.Database, *gin.Engine, func()) {
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
		Cors: config.CorsConfig{
			AllowOrigins: []string{"http://localhost:3000"},
			AllowMethods: []string{"GET", "POST", "OPTIONS"},
			AllowHeaders: []string{"Origin", "Content-Type", "Authorization"},
		},
	}

	database, err := db.NewDatabase(cfg)
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("NewDatabase: %v", err)
	}

	config.Conf = cfg

	gin.SetMode(gin.TestMode)
	r := gin.New()
	Init(r, database)

	cleanup := func() {
		database.Close()
		os.Remove(tmpFile.Name())
	}

	return database, r, cleanup
}

func TestRouterRegistration(t *testing.T) {
	_, r, cleanup := setupTestEnv(t)
	defer cleanup()

	routes := r.Routes()
	expectedPaths := map[string]bool{
		"/health":        false,
		"/help":          false,
		"/posts":         false,
		"/post/:pid":     false,
		"/comment":       false,
		"/comments/:pid": false,
		"/media/image":   false,
	}

	for _, route := range routes {
		if _, ok := expectedPaths[route.Path]; ok {
			expectedPaths[route.Path] = true
		}
	}

	for path, found := range expectedPaths {
		if !found {
			t.Errorf("Route %s not registered", path)
		}
	}
}

func TestHealthEndpoint(t *testing.T) {
	_, r, cleanup := setupTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Status = %d, want 200", w.Code)
	}
}

func TestPostsEndpoint(t *testing.T) {
	_, r, cleanup := setupTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/posts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Status = %d, want 200", w.Code)
	}
}

func TestPostEndpointInvalidPid(t *testing.T) {
	_, r, cleanup := setupTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/post/abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("Status = %d, want 400", w.Code)
	}
}

func TestCommentsEndpointInvalidPid(t *testing.T) {
	_, r, cleanup := setupTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/comments/abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("Status = %d, want 400", w.Code)
	}
}

func TestCommentEndpointInvalidCid(t *testing.T) {
	_, r, cleanup := setupTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/comment?cid=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("Status = %d, want 400", w.Code)
	}
}

func TestHelpEndpoint(t *testing.T) {
	_, r, cleanup := setupTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/help", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("Status = %d, want 200", w.Code)
	}
}

func TestMediaImageEndpointNoParam(t *testing.T) {
	_, r, cleanup := setupTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/media/image", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("Status = %d, want 400", w.Code)
	}
}

func TestCORSHeaders(t *testing.T) {
	_, r, cleanup := setupTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("OPTIONS", "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Errorf("OPTIONS Status = %d, want 204", w.Code)
	}

	ao := w.Header().Get("Access-Control-Allow-Origin")
	if ao != "http://localhost:3000" {
		t.Errorf("Access-Control-Allow-Origin = %s, want http://localhost:3000", ao)
	}
}

func TestCORSAllowMethods(t *testing.T) {
	_, r, cleanup := setupTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("OPTIONS", "/posts", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	methods := w.Header().Get("Access-Control-Allow-Methods")
	if methods == "" {
		t.Error("Access-Control-Allow-Methods header is missing")
	}
}

func TestEndToEndFlow(t *testing.T) {
	database, r, cleanup := setupTestEnv(t)
	defer cleanup()

	posts := []models.Post{
		{Pid: 1, Text: "End to end test post", Type: "text", Timestamp: 1000},
		{Pid: 2, Text: "Another post for testing", Type: "text", Timestamp: 2000},
	}
	if err := database.UpsertPosts(posts); err != nil {
		t.Fatalf("UpsertPosts: %v", err)
	}

	req := httptest.NewRequest("GET", "/posts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /posts Status = %d, want 200", w.Code)
	}

	req = httptest.NewRequest("GET", "/post/1", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /post/1 Status = %d, want 200", w.Code)
	}

	req = httptest.NewRequest("GET", "/posts?keyword=end", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /posts?keyword=end Status = %d, want 200", w.Code)
	}
}
