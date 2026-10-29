package crawler

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"treehole/internal/config"
	"treehole/internal/db"
	"treehole/internal/models"
)

func TestCrawlSessionDeletionRequiresSuccessfulAdjacentPages(t *testing.T) {
	for _, tc := range []struct {
		name        string
		failPageTwo bool
		secondPage  int
		wantDeleted bool
	}{
		{name: "adjacent pages", secondPage: 2, wantDeleted: true},
		{name: "skipped page", secondPage: 3},
		{name: "failed page", failPageTwo: true, secondPage: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			database, err := db.NewDatabase(&config.Config{Database: config.DatabaseConfig{
				Type: "sqlite3", DBFile: filepath.Join(t.TempDir(), "crawl.db"),
			}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = database.Close() })
			if err := database.UpsertPosts([]models.Post{{Pid: 90, Text: "previously seen"}}); err != nil {
				t.Fatal(err)
			}
			c := mustNewTestClient(t)
			c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
				body := `{"code":20000,"data":{"total":0,"list":[]}}`
				if !strings.HasSuffix(r.URL.Path, "/comment/list") {
					page := r.URL.Query().Get("page")
					switch {
					case page == "1":
						body = `{"code":20000,"data":{"list":[{"pid":100}]}}`
					case page == "2" && tc.failPageTwo:
						body = `{"code":40000,"message":"page unavailable"}`
					default:
						body = `{"code":20000,"data":{"list":[{"pid":80}]}}`
					}
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			session := NewCrawlSession()
			fetch := func(page int) error {
				_, err := session.FetchAndSave(c, database, page, false, 20, 2, false, false)
				return err
			}
			if err := fetch(1); err != nil {
				t.Fatal(err)
			}
			if tc.failPageTwo {
				if err := fetch(2); err == nil {
					t.Fatal("failed page was accepted")
				}
				if session.previousPagePosts != nil {
					t.Fatal("failed page retained preceding deletion evidence")
				}
			}
			if err := fetch(tc.secondPage); err != nil {
				t.Fatal(err)
			}
			post, err := database.GetPostByPid(90)
			if err != nil {
				t.Fatal(err)
			}
			if post == nil || (post.DeletedAt != 0) != tc.wantDeleted {
				t.Fatalf("post = %+v, want deleted=%v", post, tc.wantDeleted)
			}
		})
	}
}

func TestIncompleteCommentsDoNotSavePageOrDeletionMarks(t *testing.T) {
	database, err := db.NewDatabase(&config.Config{Database: config.DatabaseConfig{Type: "sqlite3", DBFile: filepath.Join(t.TempDir(), "snapshot.db")}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.SaveCrawlResult([]models.Post{{Pid: 10, Text: "old"}}, []models.Comment{{Pid: 10, Cid: 3, Text: "preserved"}}); err != nil {
		t.Fatal(err)
	}
	c := mustNewTestClient(t)
	c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"code":20000,"data":{"list":[{"pid":10,"text":"new","reply":2}]}}`
		if strings.HasSuffix(r.URL.Path, "/comment/list") {
			body = `{"code":20000,"data":{"total":2,"list":[{"pid":10,"cid":1}]}}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	if _, err := FetchAndSave(c, database, 1, false, 20, 1, false, false); err == nil {
		t.Fatal("incomplete snapshot succeeded")
	}
	post, err := database.GetPostByPid(10)
	if err != nil {
		t.Fatal(err)
	}
	comment, err := database.GetCommentByCid(3)
	if err != nil {
		t.Fatal(err)
	}
	if post.Text != "old" || comment.Text != "preserved" || comment.DeletedAt != 0 {
		t.Fatalf("failed snapshot changed database: %+v", post)
	}
}

func TestPostDeletionRangeIgnoresPinnedAndUnorderedPosts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		posts []models.Post
		want  []int32
	}{
		{"bounded gap", []models.Post{{Pid: 100}, {Pid: 80}}, []int32{90}},
		{"pinned boundary", []models.Post{{Pid: 10, IsTop: true}, {Pid: 100}, {Pid: 80}}, []int32{90}},
		{"unordered", []models.Post{{Pid: 80}, {Pid: 100}}, nil},
		{"pinned only", []models.Post{{Pid: 100, IsTop: true}, {Pid: 10, IsTop: true}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			database, err := db.NewDatabase(&config.Config{Database: config.DatabaseConfig{Type: "sqlite3", DBFile: filepath.Join(t.TempDir(), "gaps.db")}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = database.Close() })
			if err := database.UpsertPosts([]models.Post{{Pid: 20}, {Pid: 90}, {Pid: 110}}); err != nil {
				t.Fatal(err)
			}
			missing, err := missingPostsInPage(database, tc.posts, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(missing) != len(tc.want) {
				t.Fatalf("missing=%v want=%v", missing, tc.want)
			}
			for i := range missing {
				if missing[i] != tc.want[i] {
					t.Fatalf("missing=%v want=%v", missing, tc.want)
				}
			}
		})
	}
}
