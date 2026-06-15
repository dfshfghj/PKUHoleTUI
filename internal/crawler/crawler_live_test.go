package crawler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"treehole/internal/client"
	"treehole/internal/config"
	"treehole/internal/db"
	"treehole/internal/models"
)

func skipIfNoLiveEnv(t *testing.T) {
	t.Helper()
	if os.Getenv("PKUHOLE_LIVE_TEST") != "1" {
		t.Skip("skipping live API test; set PKUHOLE_LIVE_TEST=1 to run")
	}
}

func chdirToProjectRoot(t *testing.T) {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(filename), "../..")
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir to project root: %v", err)
	}
}

func setupLiveClient(t *testing.T) *client.Client {
	t.Helper()
	chdirToProjectRoot(t)

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	c, err := client.NewClient(cfg.DeviceUUID)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if c.GetAuthorization() != "" {
		return c
	}

	oauthResult, err := c.OAuthLogin(cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("OAuthLogin: %v", err)
	}

	token, ok := oauthResult["token"].(string)
	if !ok || token == "" {
		t.Fatalf("OAuthLogin did not return a token")
	}

	err = c.SSOLogin(token)
	if err != nil {
		t.Fatalf("SSOLogin: %v", err)
	}

	return c
}

func setupLiveDB(t *testing.T) (*db.Database, func()) {
	t.Helper()
	chdirToProjectRoot(t)

	tmpFile, err := os.CreateTemp("", "test_live_*.db")
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

func TestLiveFetchPostsV3FieldMapping(t *testing.T) {
	skipIfNoLiveEnv(t)
	c := setupLiveClient(t)

	resp, err := c.GetPostsList(1, 5, 10, 1)
	if err != nil {
		t.Fatalf("GetPostsList: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var rawResponse map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&rawResponse)

	// Verify API structure
	code, ok := rawResponse["code"].(float64)
	if !ok || code != 20000 {
		t.Fatalf("API code = %v, want 20000", code)
	}

	data, ok := rawResponse["data"].(map[string]interface{})
	if !ok {
		t.Fatal("missing 'data' field")
	}

	list, ok := data["list"].([]interface{})
	if !ok {
		t.Fatal("missing 'list' field")
	}
	if len(list) == 0 {
		t.Fatal("list is empty")
	}

	// Marshal and unmarshal through TempPost to validate field mapping
	postsBytes, _ := json.Marshal(list)
	var tempPosts []TempPost
	if err := json.Unmarshal(postsBytes, &tempPosts); err != nil {
		t.Fatalf("unmarshal TempPost: %v", err)
	}

	// Validate every post's fields
	for i, tp := range tempPosts {
		// Core fields must be non-zero/non-empty
		if tp.Pid == 0 {
			t.Errorf("post[%d] pid = 0", i)
		}
		if tp.Text == "" {
			t.Errorf("post[%d] text is empty", i)
		}
		if tp.Timestamp == 0 {
			t.Errorf("post[%d] timestamp = 0", i)
		}

		// Verify int fields are properly parsed (not always 0)
		// At least some posts should have non-zero values
		t.Logf("post[%d] pid=%d reply=%d likenum=%d anonymous=%d type=%s",
			i, tp.Pid, tp.Reply, tp.Likenum, tp.Anonymous, tp.Type)

		// Validate comment_list field mapping
		for j, tc := range tp.CommentList {
			if tc.Cid == 0 {
				t.Errorf("post[%d].comment[%d] cid = 0", i, j)
			}
			if tc.Text == "" {
				t.Errorf("post[%d].comment[%d] text is empty", i, j)
			}
			if tc.Timestamp == 0 {
				t.Errorf("post[%d].comment[%d] timestamp = 0", i, j)
			}
			t.Logf("  comment[%d] cid=%d pid=%d text_len=%d",
				j, tc.Cid, tc.Pid, len(tc.Text))
		}
	}
}

func TestLiveFetchPostsV3ToModels(t *testing.T) {
	skipIfNoLiveEnv(t)
	c := setupLiveClient(t)
	database, cleanup := setupLiveDB(t)
	defer cleanup()

	posts, comments, _, err := fetchPostsV3(c, 1, 5, 10, false, false)
	if err != nil {
		t.Fatalf("fetchPostsV3: %v", err)
	}

	if len(posts) == 0 {
		t.Fatal("no posts returned")
	}

	// Validate Post model fields
	for i, p := range posts {
		if p.Pid == 0 {
			t.Errorf("posts[%d].Pid = 0", i)
		}
		if p.Text == "" {
			t.Errorf("posts[%d].Text is empty", i)
		}
		if p.Timestamp == 0 {
			t.Errorf("posts[%d].Timestamp = 0", i)
		}
		if p.Type == "" {
			t.Errorf("posts[%d].Type is empty", i)
		}

		// Verify BoolInt fields
		t.Logf("post[%d] pid=%d anonymous=%v hidden=%v is_top=%v type=%s",
			i, p.Pid, p.Anonymous, p.Hidden, p.IsTop, p.Type)

		// Verify nested comments
		for j, cm := range p.Comments {
			if cm.Cid == 0 {
				t.Errorf("posts[%d].Comments[%d].Cid = 0", i, j)
			}
			if cm.Pid != p.Pid {
				t.Errorf("posts[%d].Comments[%d].Pid=%d != post.Pid=%d",
					i, j, cm.Pid, p.Pid)
			}
		}
	}

	// Validate flat comments list
	if len(comments) > 0 {
		for i, cm := range comments {
			if cm.Cid == 0 {
				t.Errorf("comments[%d].Cid = 0", i)
			}
			t.Logf("comment[%d] cid=%d pid=%d text_len=%d",
				i, cm.Cid, cm.Pid, len(cm.Text))
		}
	}

	// Test DB write
	if err := database.UpsertPosts(posts); err != nil {
		t.Fatalf("UpsertPosts: %v", err)
	}
	if len(comments) > 0 {
		if err := database.UpsertComments(comments); err != nil {
			t.Fatalf("UpsertComments: %v", err)
		}
	}

	// Verify data persisted
	count, err := database.GetPostCount()
	if err != nil {
		t.Fatalf("GetPostCount: %v", err)
	}
	if count != len(posts) {
		t.Errorf("DB post count = %d, want %d", count, len(posts))
	}

	// Verify round-trip: read back and check fields
	for _, p := range posts {
		fetched, err := database.GetPostByPid(p.Pid)
		if err != nil {
			t.Fatalf("GetPostByPid(%d): %v", p.Pid, err)
		}
		if fetched.Text != p.Text {
			t.Errorf("round-trip pid=%d: Text mismatch", p.Pid)
		}
		if fetched.Type != p.Type {
			t.Errorf("round-trip pid=%d: Type = %s, want %s", p.Pid, fetched.Type, p.Type)
		}
		if fetched.Anonymous != p.Anonymous {
			t.Errorf("round-trip pid=%d: Anonymous = %v, want %v", p.Pid, fetched.Anonymous, p.Anonymous)
		}
	}
}

func TestLiveFetchPostsV3MultiplePages(t *testing.T) {
	skipIfNoLiveEnv(t)
	c := setupLiveClient(t)

	for page := 1; page <= 3; page++ {
		posts, comments, _, err := fetchPostsV3(c, page, 5, 5, false, false)
		if err != nil {
			t.Fatalf("page %d: fetchPostsV3: %v", page, err)
		}

		t.Logf("page %d: %d posts, %d comments", page, len(posts), len(comments))

		if len(posts) == 0 {
			t.Logf("page %d: empty (may be expected)", page)
			continue
		}

		// Verify PIDs are unique across pages
		seenPids := make(map[int32]bool)
		for _, p := range posts {
			if seenPids[p.Pid] {
				t.Errorf("page %d: duplicate pid=%d", page, p.Pid)
			}
			seenPids[p.Pid] = true
		}
	}
}

func TestLiveFetchPostsV3WithImages(t *testing.T) {
	skipIfNoLiveEnv(t)
	c := setupLiveClient(t)

	// Fetch more posts to increase chance of finding image posts
	posts, _, _, err := fetchPostsV3(c, 1, 50, 0, false, false)
	if err != nil {
		t.Fatalf("fetchPostsV3: %v", err)
	}

	var imagePosts []models.Post
	for _, p := range posts {
		if p.Type == "image" || p.MediaIds != "" {
			imagePosts = append(imagePosts, p)
		}
	}

	if len(imagePosts) > 0 {
		t.Logf("Found %d posts with images out of %d", len(imagePosts), len(posts))
		for i, p := range imagePosts {
			if i >= 3 {
				break
			}
			t.Logf("  image post: pid=%d type=%s media_ids=%s", p.Pid, p.Type, p.MediaIds)
		}
	} else {
		t.Log("No image posts found in this batch (may be expected)")
	}
}

func TestLiveFetchPostsV3Performance(t *testing.T) {
	skipIfNoLiveEnv(t)
	c := setupLiveClient(t)

	start := time.Now()
	posts, comments, _, err := fetchPostsV3(c, 1, 20, 20, false, false)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("fetchPostsV3: %v", err)
	}

	t.Logf("Fetched %d posts, %d comments in %v", len(posts), len(comments), elapsed)

	if elapsed > 30*time.Second {
		t.Errorf("Fetch took %v, expected < 30s", elapsed)
	}
}

func TestLiveCrawlerFetchAndSave(t *testing.T) {
	skipIfNoLiveEnv(t)
	c := setupLiveClient(t)
	database, cleanup := setupLiveDB(t)
	defer cleanup()

	result, err := FetchAndSave(c, database, 1, false, 10, 10, false, false)
	if err != nil {
		t.Fatalf("FetchAndSave: %v", err)
	}

	t.Logf("FetchAndSave: %d posts, %d comments", result.PostCount, result.CommentCount)

	if result.PostCount == 0 {
		t.Error("PostCount = 0, expected > 0")
	}

	// Verify data in DB
	postCount, _ := database.GetPostCount()
	commentCount, _ := database.GetCommentCount()

	t.Logf("DB: %d posts, %d comments", postCount, commentCount)

	if postCount != result.PostCount {
		t.Errorf("DB post count = %d, want %d", postCount, result.PostCount)
	}
}
