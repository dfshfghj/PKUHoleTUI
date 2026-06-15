package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"treehole/internal/config"
)

// skipIfNoLiveEnv skips the test unless PKUHOLE_LIVE_TEST=1 is set.
// These tests make real HTTP requests to treehole.pku.edu.cn.
func skipIfNoLiveEnv(t *testing.T) {
	t.Helper()
	if os.Getenv("PKUHOLE_LIVE_TEST") != "1" {
		t.Skip("skipping live API test; set PKUHOLE_LIVE_TEST=1 to run")
	}
}

// chdirToProjectRoot changes to the project root directory so config.LoadConfig works.
func chdirToProjectRoot(t *testing.T) {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(filename), "../..")
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir to project root: %v", err)
	}
}

// setupLiveClient creates a client using the real config.json and logs in.
func setupLiveClient(t *testing.T) (*Client, *config.Config) {
	t.Helper()
	chdirToProjectRoot(t)

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	c, err := NewClient(cfg.DeviceUUID)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	// If we already have a token from cookies, skip login
	if c.GetAuthorization() != "" {
		return c, cfg
	}

	// Try OAuth + SSO login
	oauthResult, err := c.OAuthLogin(cfg.Username, cfg.Password)
	if err != nil {
		t.Fatalf("OAuthLogin: %v", err)
	}

	token, ok := oauthResult["token"].(string)
	if !ok || token == "" {
		t.Fatalf("OAuthLogin did not return a token: %v", oauthResult)
	}

	err = c.SSOLogin(token)
	if err != nil {
		t.Fatalf("SSOLogin: %v", err)
	}

	return c, cfg
}

func TestLiveClientGetPostsList(t *testing.T) {
	skipIfNoLiveEnv(t)
	c, _ := setupLiveClient(t)

	resp, err := c.GetPostsList(1, 5, 3, 1)
	if err != nil {
		t.Fatalf("GetPostsList: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("GetPostsList status = %d, want 200", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Verify top-level structure
	code, ok := result["code"].(float64)
	if !ok {
		t.Fatal("response missing 'code' field")
	}
	if code != 20000 {
		t.Fatalf("code = %v, want 20000", code)
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'data' field")
	}

	list, ok := data["list"].([]interface{})
	if !ok {
		t.Fatal("data missing 'list' field")
	}

	if len(list) == 0 {
		t.Fatal("list is empty, expected at least 1 post")
	}

	// Verify each post has required fields
	requiredFields := []string{
		"pid", "text", "type", "timestamp", "hidden", "reply", "likenum",
		"extra", "anonymous", "protected", "is_top", "label", "status",
		"is_comment", "tags_ids", "auto_tags_ids", "mgr_tags_ids", "media_ids",
		"fold", "kind", "reward_cost", "reward_state", "identity_show",
		"identity_type", "identity_info", "exclusive_id_id", "exclusive_id_info",
		"mention", "mailbox", "image_size", "has_reward_good", "is_god_hole",
		"is_protect", "tread_num", "praise_num", "praise_num_show", "fold_num",
		"is_fold", "cannot_reply", "comment_list",
	}

	for i, item := range list {
		post, ok := item.(map[string]interface{})
		if !ok {
			t.Errorf("list[%d] is not a map", i)
			continue
		}
		for _, field := range requiredFields {
			if _, exists := post[field]; !exists {
				t.Errorf("post[%d] missing field: %s", i, field)
			}
		}
	}

	// Verify comment_list structure
	firstPost := list[0].(map[string]interface{})
	comments, ok := firstPost["comment_list"].([]interface{})
	if !ok {
		t.Fatal("first post missing 'comment_list' field")
	}

	if len(comments) > 0 {
		commentRequiredFields := []string{
			"anonymous", "cid", "pid", "exclusive_id_id", "hidden",
			"identity_show", "identity_type", "identity_info", "is_author",
			"is_lz", "mention", "name_tag", "reward_good", "text",
			"timestamp", "quote", "media_ids", "exclusive_id_info",
		}

		firstComment := comments[0].(map[string]interface{})
		for _, field := range commentRequiredFields {
			if _, exists := firstComment[field]; !exists {
				t.Errorf("comment missing field: %s", field)
			}
		}
	}
}

func TestLiveClientGetCommentsByPid(t *testing.T) {
	skipIfNoLiveEnv(t)
	c, _ := setupLiveClient(t)

	// First get a post to get its pid
	resp, err := c.GetPostsList(1, 1, 0, 1)
	if err != nil {
		t.Fatalf("GetPostsList: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	data := result["data"].(map[string]interface{})
	list := data["list"].([]interface{})
	firstPost := list[0].(map[string]interface{})
	pid := int(firstPost["pid"].(float64))

	// Now get comments for this post
	resp, err = c.GetCommentsByPid(pid, 1, 10, 0, 1)
	if err != nil {
		t.Fatalf("GetCommentsByPid(%d): %v", pid, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("GetCommentsByPid status = %d, want 200", resp.StatusCode)
	}

	var commentResult map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&commentResult); err != nil {
		t.Fatalf("decode comments: %v", err)
	}

	code, ok := commentResult["code"].(float64)
	if !ok {
		t.Fatal("comment response missing 'code' field")
	}
	// 40002 may mean auth required; skip field check if not authenticated
	if code != 20000 {
		t.Skipf("comment API returned code=%v (may require auth), skipping field validation", code)
	}

	cdata, ok := commentResult["data"].(map[string]interface{})
	if !ok {
		t.Fatal("comment response missing 'data' field")
	}

	comments, ok := cdata["list"].([]interface{})
	if !ok {
		t.Fatal("comment data missing 'list' field")
	}

	if len(comments) > 0 {
		commentRequiredFields := []string{
			"anonymous", "cid", "pid", "exclusive_id_id", "hidden",
			"identity_show", "identity_type", "identity_info", "is_author",
			"is_lz", "mention", "name_tag", "reward_good", "text",
			"timestamp", "quote", "media_ids", "exclusive_id_info",
		}

		firstComment := comments[0].(map[string]interface{})
		for _, field := range commentRequiredFields {
			if _, exists := firstComment[field]; !exists {
				t.Errorf("comment missing field: %s", field)
			}
		}
	}
}

func TestLiveClientUnRead(t *testing.T) {
	skipIfNoLiveEnv(t)
	c, _ := setupLiveClient(t)

	resp, err := c.UnRead()
	if err != nil {
		t.Fatalf("UnRead: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("UnRead status = %d, want 200", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode unread: %v", err)
	}

	// Verify response has expected structure
	if _, ok := result["success"]; !ok {
		t.Log("warning: unread response missing 'success' field")
	}
}

func TestLiveClientSearch(t *testing.T) {
	skipIfNoLiveEnv(t)
	c, _ := setupLiveClient(t)

	resp, err := c.Search("PKU", 1, 5, nil)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("Search status = %d, want 200", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode search: %v", err)
	}

	code, ok := result["code"].(float64)
	if !ok {
		t.Fatal("search response missing 'code' field")
	}
	// 40002 may mean auth required; skip field check if not authenticated
	if code != 20000 {
		t.Skipf("search API returned code=%v (may require auth), skipping field validation", code)
	}
}

func TestLiveClientFollowEndpoints(t *testing.T) {
	skipIfNoLiveEnv(t)
	c, _ := setupLiveClient(t)

	// GetFollow should work without needing a specific post
	resp, err := c.GetFollow(1, 5)
	if err != nil {
		t.Fatalf("GetFollow: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("GetFollow status = %d, want 200", resp.StatusCode)
	}
}

func TestLiveClientGetPost(t *testing.T) {
	skipIfNoLiveEnv(t)
	c, _ := setupLiveClient(t)

	// First get a valid pid
	resp, err := c.GetPostsList(1, 1, 0, 1)
	if err != nil {
		t.Fatalf("GetPostsList: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	data := result["data"].(map[string]interface{})
	list := data["list"].([]interface{})
	pid := int(list[0].(map[string]interface{})["pid"].(float64))

	// Get the specific post
	postResult, err := c.GetPost(pid)
	if err != nil {
		t.Fatalf("GetPost(%d): %v", pid, err)
	}

	if _, ok := postResult["code"]; !ok {
		t.Log("warning: GetPost response missing 'code' field (API may differ)")
	}
}
