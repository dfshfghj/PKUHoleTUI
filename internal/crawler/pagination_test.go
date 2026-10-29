package crawler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type crawlTransport func(*http.Request) (*http.Response, error)

func (f crawlTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestFetchPostsPaginatesAllComments(t *testing.T) {
	for _, tc := range []struct {
		name      string
		limit     int
		reply     int
		embedded  int
		wantPages []string
		wantCount int
	}{
		// embedded 长度等于 limit 时无法排除截断，补一次空页请求。
		{"embedded complete", 2, 2, 2, []string{"2"}, 2},
		{"embedded truncated", 2, 5, 2, []string{"2", "3"}, 5},
		{"no embedded comments", 0, 5, 0, []string{"1"}, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := mustNewTestClient(t)
			var commentPages []string
			c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
				if strings.HasSuffix(r.URL.Path, "/comment/list") {
					page, _ := strconv.Atoi(r.URL.Query().Get("page"))
					limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
					commentPages = append(commentPages, r.URL.Query().Get("page"))
					items := make([]map[string]int, 0, limit)
					for cid := (page-1)*limit + 1; cid <= page*limit && cid <= tc.reply; cid++ {
						items = append(items, map[string]int{"pid": 10, "cid": cid})
					}
					body, _ := json.Marshal(map[string]interface{}{"code": 20000, "data": map[string]interface{}{"total": tc.reply, "list": items}})
					return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
				}
				if got := r.URL.Query().Get("comment_limit"); got != fmt.Sprint(tc.limit) {
					t.Errorf("comment_limit=%s, want %d", got, tc.limit)
				}
				embedded := make([]map[string]int, 0, tc.embedded)
				for cid := 1; cid <= tc.embedded; cid++ {
					embedded = append(embedded, map[string]int{"pid": 10, "cid": cid})
				}
				body, _ := json.Marshal(map[string]interface{}{"code": 20000, "data": map[string]interface{}{
					"list": []map[string]interface{}{{"pid": 10, "reply": tc.reply, "comment_list": embedded}},
				}})
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
			})
			posts, comments, _, err := fetchPostsV3(c, 1, 20, tc.limit, false, false)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(commentPages, tc.wantPages) {
				t.Fatalf("comment pages=%v want=%v", commentPages, tc.wantPages)
			}
			if len(comments) != tc.wantCount || len(posts[0].Comments) != tc.wantCount {
				t.Fatalf("comments=%d post comments=%d want=%d", len(comments), len(posts[0].Comments), tc.wantCount)
			}
			for i, comment := range comments {
				if comment.Cid != int32(i+1) {
					t.Fatalf("comment[%d]=%d", i, comment.Cid)
				}
			}
		})
	}
}

func TestFetchPostsRejectsIncompleteComments(t *testing.T) {
	for _, response := range []string{
		`{"code":20000,"data":{"total":2,"list":[{"cid":1,"pid":10}]}}`,
		`{"code":20000,"data":{}}`,
		`{"code":40000,"message":"denied"}`,
	} {
		t.Run(response, func(t *testing.T) {
			c := mustNewTestClient(t)
			c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
				body := `{"code":20000,"data":{"list":[{"pid":10,"reply":2}]}}`
				if strings.HasSuffix(r.URL.Path, "/comment/list") {
					body = response
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			if _, _, _, err := fetchPostsV3(c, 1, 20, 1, false, false); err == nil {
				t.Fatal("incomplete comments accepted")
			}
		})
	}
}

func TestFetchCommentsRejectsForeignIDs(t *testing.T) {
	for _, tc := range []struct{ name, second string }{
		{"foreign post", `{"code":20000,"data":{"total":2,"list":[{"pid":11,"cid":2}]}}`},
		{"invalid id", `{"code":20000,"data":{"total":2,"list":[{"pid":10,"cid":0}]}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := mustNewTestClient(t)
			c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
				body := `{"code":20000,"data":{"total":2,"list":[{"pid":10,"cid":1}]}}`
				if r.URL.Query().Get("page") == "2" {
					body = tc.second
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			if _, err := fetchAllComments(c, 10, 1); err == nil {
				t.Fatal("invalid snapshot accepted")
			}
		})
	}
}

func TestFetchCommentsIgnoresResponseTotal(t *testing.T) {
	c := mustNewTestClient(t)
	c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"code":20000,"data":{"total":0,"list":[]}}`
		switch r.URL.Query().Get("page") {
		case "1":
			body = `{"code":20000,"data":{"total":1,"list":[{"pid":10,"cid":1}]}}`
		case "2":
			body = `{"code":20000,"data":{"total":99,"list":[{"pid":10,"cid":2}]}}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	comments, err := fetchAllComments(c, 10, 1)
	if err != nil || len(comments) != 2 {
		t.Fatalf("comments=%v err=%v", comments, err)
	}
}
