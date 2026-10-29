package crawler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"treehole/internal/db"
	"treehole/internal/models"
)

func TestEstimatePostPage(t *testing.T) {
	for _, tt := range []struct {
		name           string
		target, latest int32
		limit          int
		sample         db.PIDSample
		want           int
		density        float64
	}{
		{"no history", 100, 1000, 100, db.PIDSample{}, 10, 0},
		{"half deleted", 100, 1000, 100, db.PIDSample{MinPID: 101, MaxPID: 1000, LivePosts: 450}, 5, .5},
		{"newer PID", 1001, 1000, 100, db.PIDSample{}, 1, 0},
		{"int32 boundary", 1, 2147483647, 1, db.PIDSample{}, 2147483647, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EstimatePostPage(tt.target, tt.latest, tt.limit, tt.sample)
			if err != nil || got.Page != tt.want || got.DeletedDensity != tt.density {
				t.Fatalf("estimate=%+v err=%v", got, err)
			}
		})
	}
	for _, sample := range []db.PIDSample{{LivePosts: 1}, {MinPID: 10, MaxPID: 9}, {MinPID: 1, MaxPID: 10, LivePosts: 11}, {LivePosts: -1}} {
		if _, err := EstimatePostPage(1, 100, 10, sample); err == nil {
			t.Fatalf("accepted invalid sample %+v", sample)
		}
	}
	if _, err := EstimatePostPage(1, 100, 0, db.PIDSample{}); err == nil {
		t.Fatal("accepted zero limit")
	}
}

func TestLocatePostPageCorrectsSparseHistoryAndDeletedGaps(t *testing.T) {
	// Arbitrary gaps ensure PID arithmetic alone cannot identify the page.
	pids := []int32{1000, 998, 995, 990, 980, 900, 899, 800, 500, 100, 10}
	for _, tt := range []struct {
		target int32
		page   int
	}{
		{1000, 1}, {998, 1}, {995, 2}, {996, 2}, {901, 3}, {500, 5}, {10, 6}, {1, 6}, {1001, 1},
	} {
		t.Run(strconv.Itoa(int(tt.target)), func(t *testing.T) {
			c := mustNewTestClient(t)
			calls := 0
			c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				q := r.URL.Query()
				page, _ := strconv.Atoi(q.Get("page"))
				limit, _ := strconv.Atoi(q.Get("limit"))
				if limit != 2 || q.Get("comment_limit") != "0" {
					t.Fatalf("unexpected request: %s", r.URL)
				}
				posts := []models.Post{}
				for i := (page - 1) * limit; i < page*limit && i < len(pids); i++ {
					posts = append(posts, models.Post{Pid: pids[i]})
				}
				if len(posts) > 0 {
					posts = append([]models.Post{{Pid: 3, IsTop: true}}, posts...)
				}
				body, _ := json.Marshal(map[string]any{"code": 20000, "data": map[string]any{"list": posts}})
				return retryResponse(200, string(body)), nil
			})
			page, err := LocatePostPage(c, nil, tt.target, 1000, 2)
			if err != nil || page != tt.page || calls > 24 {
				t.Fatalf("page=%d want=%d calls=%d err=%v", page, tt.page, calls, err)
			}
		})
	}
}

func TestLocatePostPageStopsOnFirstErrorOrUnorderedPage(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		err        error
	}{
		{"timeout", "", context.DeadlineExceeded},
		{"credentials", `{"code":401,"msg":"expired"}`, nil},
		{"unordered", `{"code":20000,"data":{"list":[{"pid":10},{"pid":20}]}}`, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := mustNewTestClient(t)
			calls := 0
			c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if tt.err != nil {
					return nil, tt.err
				}
				return retryResponse(200, tt.body), nil
			})
			if _, err := LocatePostPage(c, nil, 5, 100, 10); err == nil || calls != 1 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
}
