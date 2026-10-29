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

func TestCommentServerTimeoutRecovery(t *testing.T) {
	for _, tc := range []struct {
		name     string
		limit    int
		total    int
		cap      int
		statuses map[string]int
		want     []string
		wantErr  bool
	}{
		{
			name: "odd boundary overlaps accepted prefix", limit: 5, total: 12,
			statuses: map[string]int{"2/5": 504},
			want:     []string{"1/5", "2/5", "3/2", "4/2", "5/2", "6/2", "7/2"},
		},
		{
			name: "first page timeout", limit: 5, total: 7,
			statuses: map[string]int{"1/5": 408},
			want:     []string{"1/5", "1/2", "2/2", "3/2", "4/2"},
		},
		{
			name: "server cap sets effective offset", limit: 200, total: 5, cap: 2,
			statuses: map[string]int{"2/2": 504},
			want:     []string{"1/200", "2/2", "3/1", "4/1", "5/1", "6/1"},
		},
		{
			name: "auth after timeout exits", limit: 5, total: 7,
			statuses: map[string]int{"1/5": 504, "1/2": 401},
			want:     []string{"1/5", "1/2"}, wantErr: true,
		},
		{
			name: "limit one exits", limit: 2, total: 7,
			statuses: map[string]int{"1/2": 504, "1/1": 504},
			want:     []string{"1/2", "1/1"}, wantErr: true,
		},
		{
			name: "global reduction budget exits", limit: 32, total: 7,
			statuses: map[string]int{"1/32": 504, "1/16": 504, "1/8": 504, "1/4": 504},
			want:     []string{"1/32", "1/16", "1/8", "1/4"}, wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := mustNewTestClient(t)
			var requests []string
			c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
				key := fmt.Sprintf("%d/%d", page, limit)
				requests = append(requests, key)
				status := tc.statuses[key]
				if status == 0 {
					status = http.StatusOK
				}
				if tc.cap > 0 {
					limit = min(limit, tc.cap)
				}
				items := make([]map[string]int, 0)
				for i := (page - 1) * limit; i < min(page*limit, tc.total); i++ {
					items = append(items, map[string]int{"pid": 10, "cid": i + 1})
				}
				body, _ := json.Marshal(map[string]interface{}{"code": 20000, "data": map[string]interface{}{"total": tc.total, "list": items}})
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body)))}, nil
			})
			comments, err := fetchAllComments(c, 10, tc.limit)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if !reflect.DeepEqual(requests, tc.want) {
				t.Fatalf("requests=%v want=%v", requests, tc.want)
			}
			if !tc.wantErr {
				if len(comments) != tc.total {
					t.Fatalf("got %d comments, want %d", len(comments), tc.total)
				}
				for i, comment := range comments {
					if comment.Cid != int32(i+1) {
						t.Fatalf("comment[%d]=%d", i, comment.Cid)
					}
				}
			}
		})
	}
}

func TestCommentRecoveryRejectsChangedOverlap(t *testing.T) {
	c := mustNewTestClient(t)
	requests := 0
	c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		status := 200
		body := `{"code":20000,"data":{"total":8,"list":[{"pid":10,"cid":1},{"pid":10,"cid":2},{"pid":10,"cid":3},{"pid":10,"cid":4},{"pid":10,"cid":5}]}}`
		if requests == 2 {
			status = 504
		}
		if requests == 3 {
			body = `{"code":20000,"data":{"total":8,"list":[{"pid":10,"cid":999},{"pid":10,"cid":6}]}}`
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	if _, err := fetchAllComments(c, 10, 5); err == nil || !strings.Contains(err.Error(), "boundary changed") {
		t.Fatalf("changed overlap accepted: %v", err)
	}
	if requests != 3 {
		t.Fatalf("requests=%d", requests)
	}
}

func TestCommentsStopOnShortPage(t *testing.T) {
	c := mustNewTestClient(t)
	requests := 0
	c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		body := `{"code":20000,"data":{"total":5,"list":[{"pid":10,"cid":1},{"pid":10,"cid":2}]}}`
		if requests == 2 {
			body = `{"code":20000,"data":{"total":5,"list":[{"pid":10,"cid":3}]}}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	comments, err := fetchAllComments(c, 10, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 3 || comments[2].Cid != 3 {
		t.Fatalf("comments=%v", comments)
	}
	if requests != 2 {
		t.Fatalf("requests=%d", requests)
	}
}

// 零评论帖子的 total 会等于请求的 limit，不能据此判定分页不完整。
func TestCommentsEmptyFirstPageIsComplete(t *testing.T) {
	c := mustNewTestClient(t)
	requests := 0
	c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		body := `{"code":20000,"data":{"total":100,"list":[]}}`
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	comments, err := fetchAllComments(c, 10, 200)
	if err != nil || len(comments) != 0 {
		t.Fatalf("comments=%v err=%v", comments, err)
	}
	if requests != 1 {
		t.Fatalf("requests=%d", requests)
	}
}
