package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"treehole/internal/db"
	"treehole/internal/models"

	"github.com/gin-gonic/gin"
)

func getJSON(t *testing.T, r *gin.Engine, path string) map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("GET %s status = %d body=%s", path, w.Code, w.Body.String())
	}
	var envelope map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("GET %s invalid json: %v", path, err)
	}
	if envelope["code"].(float64) != 20000 || envelope["success"] != true {
		t.Fatalf("GET %s envelope = %v", path, envelope)
	}
	if _, ok := envelope["timestamp"].(float64); !ok {
		t.Fatalf("GET %s missing timestamp", path)
	}
	return envelope
}

func seedV3Posts(t *testing.T, database *db.Database) {
	t.Helper()
	quote := int32(11)
	err := database.SaveCrawlResult([]models.Post{
		{Pid: 1001, Text: "保研经验分享清华路线", Type: "text", Timestamp: 1000, Reply: 2, Likenum: 5},
		{Pid: 1002, Text: "考研经验分享北大路线", Type: "text", Timestamp: 2000, Reply: 1},
	}, []models.Comment{
		{Cid: 11, Pid: 1001, Text: "第一楼", Timestamp: 1100, NameTag: ""},
		{Cid: 12, Pid: 1001, Text: "引用了第一楼", Timestamp: 1200, QuoteID: &quote},
		{Cid: 21, Pid: 1002, Text: "考研加油", Timestamp: 2100, NameTag: "Bob"},
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func assertKeys(t *testing.T, got map[string]interface{}, want []string, ctx string) {
	t.Helper()
	for _, k := range want {
		if _, ok := got[k]; !ok {
			t.Errorf("%s missing key %q; got keys %v", ctx, k, keysOf(got))
		}
	}
}

func keysOf(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestV3ListCommentsShape(t *testing.T) {
	database, r, cleanup := setupTestEnv(t)
	defer cleanup()
	seedV3Posts(t, database)

	envelope := getJSON(t, r, "/chapi/api/v3/hole/list_comments?page=1&limit=10&comment_limit=10")
	data := envelope["data"].(map[string]interface{})
	if data["total"].(float64) != 2 {
		t.Fatalf("total = %v, want 2", data["total"])
	}
	list := data["list"].([]interface{})
	if len(list) != 2 {
		t.Fatalf("list len = %d, want 2", len(list))
	}
	hole := list[0].(map[string]interface{})
	assertKeys(t, hole, []string{
		"pid", "text", "type", "timestamp", "hidden", "reply", "likenum", "extra",
		"anonymous", "hot", "tag", "protected", "is_top", "label", "status",
		"is_comment", "tags_ids", "auto_tags_ids", "mgr_tags_ids", "media_ids",
		"fold", "kind", "reward_cost", "reward_state", "identity_show",
		"identity_type", "exclusive_id_id", "mention", "mailbox", "image_size",
		"has_reward_good", "tags_info", "tags_list", "exclusive_id_info",
		"identity_info", "is_god_hole", "is_protect", "tread_num", "praise_num",
		"praise_num_show", "fold_num", "is_sdss", "is_follow", "attention_info",
		"is_praise", "is_tread", "islz", "user_fold", "user_config_fold",
		"is_fold", "cannot_reply", "comment_list", "comment_total",
	}, "list_comments hole")
	if hole["pid"].(float64) != 1002 {
		t.Fatalf("first hole pid = %v, want 1002 (pid DESC)", hole["pid"])
	}
	cl := hole["comment_list"].([]interface{})
	if len(cl) != 1 || cl[0].(map[string]interface{})["cid"].(float64) != 21 {
		t.Fatalf("comment_list = %v", cl)
	}

	envelope = getJSON(t, r, "/chapi/api/v3/hole/list_comments?keyword=保研")
	list = envelope["data"].(map[string]interface{})["list"].([]interface{})
	if len(list) != 1 || list[0].(map[string]interface{})["pid"].(float64) != 1001 {
		t.Fatalf("keyword search results = %v", list)
	}
}

func TestV3HoleOneAndGet(t *testing.T) {
	database, r, cleanup := setupTestEnv(t)
	defer cleanup()
	seedV3Posts(t, database)

	envelope := getJSON(t, r, "/chapi/api/v3/hole/one?pid=1001")
	data := envelope["data"].(map[string]interface{})
	hole := data["hole"].(map[string]interface{})
	assertKeys(t, hole, []string{"pid", "text", "reply", "likenum", "user_config_fold"}, "one.hole")
	if data["total"].(float64) != 2 {
		t.Fatalf("one.total = %v, want 2", data["total"])
	}
	cmts := data["list"].([]interface{})
	if len(cmts) != 2 {
		t.Fatalf("one.list len = %d, want 2", len(cmts))
	}
	first := cmts[0].(map[string]interface{})
	assertKeys(t, first, []string{
		"cid", "pid", "text", "timestamp", "hidden", "anonymous", "tag",
		"comment_id", "name_tag", "media_ids", "reward_good", "identity_show",
		"identity_type", "exclusive_id_id", "mention", "quote",
		"exclusive_id_info", "identity_info", "is_author", "is_lz",
	}, "one.comment")
	if first["name_tag"] != "Alice" {
		t.Fatalf("default name_tag = %v, want Alice", first["name_tag"])
	}
	second := cmts[1].(map[string]interface{})
	if second["comment_id"].(float64) != 11 {
		t.Fatalf("comment_id = %v, want 11", second["comment_id"])
	}
	q, ok := second["quote"].(map[string]interface{})
	if !ok || q["cid"].(float64) != 11 {
		t.Fatalf("quote = %v, want nested cid 11", second["quote"])
	}

	envelope = getJSON(t, r, "/chapi/api/v3/hole/get?pid=1001")
	hole = envelope["data"].(map[string]interface{})
	if hole["pid"].(float64) != 1001 || hole["reply"].(float64) != 2 {
		t.Fatalf("hole/get data = %v", hole)
	}
	if _, ok := hole["comment_list"]; ok {
		t.Fatal("hole/get must not include comment_list")
	}

	req := httptest.NewRequest("GET", "/chapi/api/v3/hole/get?pid=999999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var envelope404 map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &envelope404)
	if envelope404["success"] != false {
		t.Fatalf("missing pid should return success=false, got %v", envelope404)
	}
}

func TestV3CommentListPagination(t *testing.T) {
	database, r, cleanup := setupTestEnv(t)
	defer cleanup()
	seedV3Posts(t, database)

	envelope := getJSON(t, r, "/chapi/api/v3/comment/list?pid=1001&page=1&limit=1&sort=0")
	data := envelope["data"].(map[string]interface{})
	if data["total"].(float64) != 2 {
		t.Fatalf("total = %v, want 2", data["total"])
	}
	list := data["list"].([]interface{})
	if len(list) != 1 || list[0].(map[string]interface{})["cid"].(float64) != 11 {
		t.Fatalf("page1 asc = %v", list)
	}

	envelope = getJSON(t, r, "/chapi/api/v3/comment/list?pid=1001&page=2&limit=1&sort=1")
	list = envelope["data"].(map[string]interface{})["list"].([]interface{})
	if len(list) != 1 || list[0].(map[string]interface{})["cid"].(float64) != 11 {
		t.Fatalf("page2 desc = %v", list)
	}
}

func TestV3KeywordWithKindFilter(t *testing.T) {
	database, r, cleanup := setupTestEnv(t)
	defer cleanup()
	seedV3Posts(t, database)

	err := database.SaveCrawlResult([]models.Post{
		{Pid: 1003, Text: "保研悬赏求助帖", Type: "text", Timestamp: 3000, Kind: 1},
		{Pid: 1004, Text: "保研经验普通帖", Type: "text", Timestamp: 4000, Kind: 0},
	}, nil)
	if err != nil {
		t.Fatalf("seed kind posts: %v", err)
	}

	envelope := getJSON(t, r, "/chapi/api/v3/hole/list_comments?page=1&limit=10&keyword=%E4%BF%9D%E7%A0%94&kind=1")
	list := envelope["data"].(map[string]interface{})["list"].([]interface{})
	if len(list) != 1 || list[0].(map[string]interface{})["pid"].(float64) != 1003 {
		t.Fatalf("keyword+kind=1 results = %v, want [1003]", pidsOf(list))
	}

	envelope = getJSON(t, r, "/chapi/api/v3/hole/list_comments?page=1&limit=10&keyword=%E4%BF%9D%E7%A0%94&kind=0")
	list = envelope["data"].(map[string]interface{})["list"].([]interface{})
	got := pidsOf(list)
	if len(got) < 1 || len(got) > 2 {
		t.Fatalf("keyword+kind=0 results = %v", got)
	}
}

func pidsOf(list []interface{}) []float64 {
	out := make([]float64, 0, len(list))
	for _, item := range list {
		out = append(out, item.(map[string]interface{})["pid"].(float64))
	}
	return out
}

func TestV3StatelessDefaults(t *testing.T) {
	_, r, cleanup := setupTestEnv(t)
	defer cleanup()

	envelope := getJSON(t, r, "/chapi/api/v3/message/un_read?message_type=int_msg")
	if envelope["data"].(map[string]interface{})["count"].(float64) != 0 {
		t.Fatal("un_read count should be 0")
	}

	envelope = getJSON(t, r, "/chapi/api/v3/message/index?page=1&limit=10&message_type=int_msg")
	data := envelope["data"].(map[string]interface{})
	if data["total"].(float64) != 0 || len(data["list"].([]interface{})) != 0 {
		t.Fatal("message index should be empty")
	}

	envelope = getJSON(t, r, "/chapi/api/v3/tags/tree")
	if tags, ok := envelope["data"].([]interface{}); !ok || len(tags) != 0 {
		t.Fatal("tags tree should be empty array")
	}
}
