package crawler

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"gorm.io/gorm"
	"treehole/internal/config"
	"treehole/internal/db"
	"treehole/internal/models"
)

// Nonconsecutive PIDs model remotely deleted posts; offsets count live posts.
var recoveryPIDs = []int32{100, 98, 97, 93, 90, 88, 85, 84, 80, 77, 74, 70, 65, 60, 55}

func recoveryBody(t *testing.T, page, limit int) string {
	t.Helper()
	items := make([]map[string]interface{}, 0, limit)
	for i := (page - 1) * limit; i < page*limit && i < len(recoveryPIDs); i++ {
		items = append(items, map[string]interface{}{"pid": recoveryPIDs[i], "reply": 0})
	}
	b, err := json.Marshal(map[string]interface{}{"code": 20000, "data": map[string]interface{}{"total": len(recoveryPIDs), "list": items}})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func recoveryIDs(t *testing.T, raw map[string]interface{}) []int32 {
	t.Helper()
	var ids []int32
	for _, item := range raw["data"].(map[string]interface{})["list"].([]interface{}) {
		ids = append(ids, int32(item.(map[string]interface{})["pid"].(float64)))
	}
	return ids
}

func recoveryDatabase(t *testing.T) *db.Database {
	t.Helper()
	d, err := db.NewDatabase(&config.Config{Database: config.DatabaseConfig{Type: "sqlite3", DBFile: filepath.Join(t.TempDir(), "recovery.db")}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestPostRecoveryPreservesOddPageBoundaries(t *testing.T) {
	c := mustNewTestClient(t)
	var requests [][2]int
	c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		requests = append(requests, [2]int{page, limit})
		if limit > 2 {
			return retryResponse(504, ""), nil
		}
		return retryResponse(200, recoveryBody(t, page, limit)), nil
	})
	raw, err := fetchPostListRecovering(c, nil, 2, 5, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := recoveryIDs(t, raw); !reflect.DeepEqual(got, recoveryPIDs[5:10]) {
		t.Fatalf("PIDs=%v want=%v", got, recoveryPIDs[5:10])
	}
	if want := [][2]int{{2, 5}, {3, 2}, {4, 2}, {5, 2}}; !reflect.DeepEqual(requests, want) {
		t.Fatalf("requests=%v want=%v", requests, want)
	}
}

func TestPostRecoveryAnchorsDespiteSparseLocalSample(t *testing.T) {
	d := recoveryDatabase(t)
	if err := d.UpsertPosts([]models.Post{{Pid: 100}, {Pid: 90}}); err != nil {
		t.Fatal(err)
	}
	c := mustNewTestClient(t)
	requests := 0
	c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit > 2 {
			return retryResponse(504, ""), nil
		}
		return retryResponse(200, recoveryBody(t, page, limit)), nil
	})
	raw, err := fetchPostListRecovering(c, d, 2, 5, 0, []models.Post{{Pid: 100}, {Pid: 90}})
	if err != nil {
		t.Fatal(err)
	}
	got := recoveryIDs(t, raw)
	for _, want := range recoveryPIDs[4:10] {
		found := false
		for _, pid := range got {
			if pid == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("anchor/boundary PID %d missing from %v", want, got)
		}
	}
	if requests > maxRecoveryPages {
		t.Fatalf("unbounded requests=%d", requests)
	}
}

func TestPostRecoveryStopsAtSmallerPageErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"HTTP credentials", 401, ""}, {"API credentials", 200, `{"code":40001,"message":"expired"}`}, {"other server error", 500, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := mustNewTestClient(t)
			attempts := 0
			c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
				attempts++
				if attempts == 1 {
					return retryResponse(504, ""), nil
				}
				return retryResponse(tc.status, tc.body), nil
			})
			if _, err := fetchPostListRecovering(c, nil, 2, 5, 0, nil); err == nil {
				t.Fatal("page failure ignored")
			}
			if attempts != 2 {
				t.Fatalf("attempts=%d want 2", attempts)
			}
		})
	}
}

func TestPostRecoveryPersistentServerTimeoutIsBounded(t *testing.T) {
	c := mustNewTestClient(t)
	attempts := 0
	c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) { attempts++; return retryResponse(504, ""), nil })
	if _, err := fetchPostListRecovering(c, nil, 3, 100, 0, nil); err == nil {
		t.Fatal("persistent timeout ignored")
	}
	if attempts != 1+maxPageReductions {
		t.Fatalf("attempts=%d want %d", attempts, 1+maxPageReductions)
	}
}

func TestPostRecoveryRejectsInvalidSubpagesWithoutSaving(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"malformed", `{"code":20000,"data":{"total":15,"list":{}}}`},
		{"short", `{"code":20000,"data":{"total":15,"list":[{"pid":85}]}}`},
		{"duplicate boundary", `{"code":20000,"data":{"total":15,"list":[{"pid":88},{"pid":84}]}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := recoveryDatabase(t)
			if err := d.UpsertPosts([]models.Post{{Pid: 85, Text: "preserved"}}); err != nil {
				t.Fatal(err)
			}
			c := mustNewTestClient(t)
			attempts := 0
			c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
				attempts++
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
				if limit > 2 {
					return retryResponse(504, ""), nil
				}
				if page == 4 {
					return retryResponse(200, tc.body), nil
				}
				return retryResponse(200, recoveryBody(t, page, limit)), nil
			})
			if _, err := FetchAndSave(c, d, 2, false, 5, 2, false, false); err == nil {
				t.Fatal("invalid recovery accepted")
			}
			post, err := d.GetPostByPid(85)
			if err != nil || post == nil || post.Text != "preserved" || post.DeletedAt != 0 {
				t.Fatalf("original post changed: %+v err=%v", post, err)
			}
			post, err = d.GetPostByPid(88)
			if !errors.Is(err, gorm.ErrRecordNotFound) || post != nil {
				t.Fatalf("partial recovery saved: %+v err=%v", post, err)
			}
			if attempts > maxRecoveryPages {
				t.Fatalf("unbounded attempts=%d", attempts)
			}
		})
	}
}

func TestPostRecoveryRecursivelyShrinksAndKeepsTerminalBoundary(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		page, limit, threshold int
		want                   []int32
	}{
		{"recursive", 2, 5, 1, recoveryPIDs[5:10]},
		{"partial terminal", 3, 7, 3, recoveryPIDs[14:]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := mustNewTestClient(t)
			attempts := 0
			c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
				attempts++
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
				if limit > tc.threshold {
					return retryResponse(http.StatusRequestTimeout, ""), nil
				}
				return retryResponse(200, recoveryBody(t, page, limit)), nil
			})
			raw, err := fetchPostListRecovering(c, nil, tc.page, tc.limit, 0, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := recoveryIDs(t, raw); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("PIDs=%v want=%v", got, tc.want)
			}
			if attempts > maxRecoveryPages {
				t.Fatalf("unbounded attempts=%d", attempts)
			}
		})
	}
}

func TestPostRecoveryLocatesAnchorAfterRemoteDeletionMovesTail(t *testing.T) {
	for _, tc := range []struct {
		name   string
		anchor int32
		empty  bool
	}{
		{"remaining older posts", 90, false},
		{"anchor already oldest", 55, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := mustNewTestClient(t)
			requests := 0
			c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
				requests++
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
				if limit > 2 {
					return retryResponse(504, ""), nil
				}
				return retryResponse(200, recoveryBody(t, page, limit)), nil
			})
			raw, err := fetchPostListRecovering(c, nil, 6, 5, 0, []models.Post{{Pid: tc.anchor}})
			if err != nil {
				t.Fatal(err)
			}
			got := recoveryIDs(t, raw)
			if tc.empty {
				if len(got) != 0 {
					t.Fatalf("tail repeated oldest anchor: %v", got)
				}
			} else {
				for _, want := range recoveryPIDs[5:] {
					found := false
					for _, pid := range got {
						if pid == want {
							found = true
						}
					}
					if !found {
						t.Fatalf("remaining older PID %d missing from %v", want, got)
					}
				}
			}
			if requests > maxRecoveryPages {
				t.Fatalf("requests=%d exceeds shared budget=%d", requests, maxRecoveryPages)
			}
		})
	}
}

func TestRecoveryFailureSavesCollectedResponses(t *testing.T) {
	dir := t.TempDir()
	oldDir := recoveryDumpDir
	recoveryDumpDir = dir
	t.Cleanup(func() { recoveryDumpDir = oldDir })

	c := mustNewTestClient(t)
	c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit > 2 || page >= 4 {
			return retryResponse(504, ""), nil
		}
		return retryResponse(200, recoveryBody(t, page, limit)), nil
	})
	if _, err := fetchPostListRecovering(c, nil, 2, 4, 0, nil); err == nil {
		t.Fatal("expected recovery failure")
	}
	matches, err := filepath.Glob(filepath.Join(dir, "recovery_failure_*.json"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("dumps=%v err=%v", matches, err)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	var dump struct {
		Page      int                      `json:"page"`
		Limit     int                      `json:"limit"`
		Error     string                   `json:"error"`
		Responses []map[string]interface{} `json:"responses"`
	}
	if err := json.Unmarshal(data, &dump); err != nil {
		t.Fatal(err)
	}
	if dump.Page != 2 || dump.Limit != 4 || dump.Error == "" || len(dump.Responses) == 0 {
		t.Fatalf("unexpected dump: %+v", dump)
	}
}

func TestPostRecoverySharedBudgetIncludesAnchorProbes(t *testing.T) {
	c := mustNewTestClient(t)
	requests := 0
	c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit > 2 {
			return retryResponse(504, ""), nil
		}
		// Every head probe is pinned, so no newest chronological PID is found.
		// The shared request budget must stop this valid-sized stream.
		return retryResponse(200, `{"code":20000,"data":{"total":100,"list":[{"pid":100,"is_top":1}]}}`), nil
	})
	if _, err := fetchPostListRecovering(c, nil, 6, 5, 0, []models.Post{{Pid: 90}}); err == nil {
		t.Fatal("unbounded pinned head accepted")
	}
	if requests != maxRecoveryPages {
		t.Fatalf("requests=%d want shared budget=%d", requests, maxRecoveryPages)
	}
}
