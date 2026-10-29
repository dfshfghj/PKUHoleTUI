package crawler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"syscall"
	"testing"
	"time"
)

func retryResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestTimeoutRetryPreservesPostRequestAndClient(t *testing.T) {
	c := mustNewTestClient(t)
	c.GetHttpClient().Timeout = time.Second
	var urls []string
	var remaining []time.Duration
	c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/comment/list") {
			return retryResponse(200, `{"code":20000,"data":{"total":0,"list":[]}}`), nil
		}
		urls = append(urls, r.URL.String())
		deadline, ok := r.Context().Deadline()
		if !ok {
			t.Fatal("request has no timeout deadline")
		}
		remaining = append(remaining, time.Until(deadline))
		if len(urls) == 1 {
			return nil, context.DeadlineExceeded
		}
		return retryResponse(200, `{"code":20000,"data":{"list":[{"pid":10,"reply":0}]}}`), nil
	})
	posts, _, _, err := fetchPostsV3(c, 3, 20, 2, false, false)
	if err != nil || len(posts) != 1 || posts[0].Pid != 10 {
		t.Fatalf("posts=%+v err=%v", posts, err)
	}
	if len(urls) != 2 || urls[0] != urls[1] {
		t.Fatalf("retried requests differ: %v", urls)
	}
	if remaining[0] <= 0 || remaining[0] > time.Second || remaining[1] < 1500*time.Millisecond || remaining[1] > 2*time.Second {
		t.Fatalf("deadline durations=%v", remaining)
	}
	if c.GetHttpClient().Timeout != time.Second {
		t.Fatalf("shared client timeout changed: %v", c.GetHttpClient().Timeout)
	}
}

func TestTimeoutRetryKeepsCompletedCommentPages(t *testing.T) {
	c := mustNewTestClient(t)
	var pages []string
	var secondURL string
	c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(r.URL.Path, "/comment/list") {
			return retryResponse(200, `{"code":20000,"data":{"list":[{"pid":10,"reply":3}]}}`), nil
		}
		q := r.URL.Query()
		pages = append(pages, q.Get("page"))
		if q.Get("pid") != "10" || q.Get("limit") != "2" {
			t.Fatalf("unexpected comment request: %s", r.URL)
		}
		if q.Get("page") == "1" {
			return retryResponse(200, `{"code":20000,"data":{"total":3,"list":[{"cid":1,"pid":10},{"cid":2,"pid":10}]}}`), nil
		}
		if len(pages) == 2 {
			secondURL = r.URL.String()
			return nil, context.DeadlineExceeded
		}
		if r.URL.String() != secondURL {
			t.Fatalf("retry changed page boundary: %s != %s", r.URL, secondURL)
		}
		return retryResponse(200, `{"code":20000,"data":{"total":3,"list":[{"cid":3,"pid":10}]}}`), nil
	})
	posts, comments, _, err := fetchPostsV3(c, 1, 20, 2, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(pages, ",") != "1,2,2" || len(posts) != 1 || len(posts[0].Comments) != 3 || len(comments) != 3 {
		t.Fatalf("pages=%v posts=%+v comments=%+v", pages, posts, comments)
	}
	for i, comment := range comments {
		if comment.Cid != int32(i+1) {
			t.Fatalf("comment %d has cid %d", i, comment.Cid)
		}
	}
}

func TestTimeoutRetryStopsAtLimitAndCapsTimeout(t *testing.T) {
	c := mustNewTestClient(t)
	c.GetHttpClient().Timeout = 5 * time.Minute
	attempts := 0
	c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
		attempts++
		deadline, ok := r.Context().Deadline()
		if !ok {
			t.Fatal("request has no timeout deadline")
		}
		want := 10 * time.Minute
		if attempts == 1 {
			want = 5 * time.Minute
		}
		if duration := time.Until(deadline); duration < want-time.Second || duration > want {
			t.Errorf("attempt %d timeout=%v, want %v", attempts, duration, want)
		}
		return nil, context.DeadlineExceeded
	})
	if _, _, _, err := fetchPostsV3(c, 1, 20, 2, false, false); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts=%d, want 3", attempts)
	}
}

func TestRetryStopsImmediatelyOnNonTimeoutErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		err    error
	}{
		{name: "unauthorized", status: 401},
		{name: "forbidden", status: 403},
		{name: "server error", status: 500},
		{name: "expired credentials", status: 200, body: `{"code":40001,"message":"credential expired"}`},
		{name: "invalid json", status: 200, body: `{`},
		{name: "connection reset", err: syscall.ECONNRESET},
		{name: "cancelled", err: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, firstTimeout := range []bool{false, true} {
				c := mustNewTestClient(t)
				attempts := 0
				c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
					attempts++
					if firstTimeout && attempts == 1 {
						return nil, context.DeadlineExceeded
					}
					if tc.err != nil {
						return nil, tc.err
					}
					return retryResponse(tc.status, tc.body), nil
				})
				if _, _, _, err := fetchPostsV3(c, 1, 20, 2, false, false); err == nil {
					t.Fatal("request error ignored")
				}
				want := 1
				if firstTimeout {
					want = 2
				}
				if attempts != want {
					t.Fatalf("firstTimeout=%v attempts=%d want=%d", firstTimeout, attempts, want)
				}
			}
		})
	}
}

type timeoutReadBody struct{ closed bool }

func (b *timeoutReadBody) Read([]byte) (int, error) { return 0, context.DeadlineExceeded }
func (b *timeoutReadBody) Close() error             { b.closed = true; return nil }

func TestTimeoutRetryClosesFailedResponseBody(t *testing.T) {
	c := mustNewTestClient(t)
	body := &timeoutReadBody{}
	attempts := 0
	c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: body}, nil
		}
		if !body.closed {
			t.Error("timed out response body was not closed before retry")
		}
		return retryResponse(200, `{"code":20000,"data":{"list":[]}}`), nil
	})
	if _, _, _, err := fetchPostsV3(c, 1, 20, 2, false, false); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || !body.closed {
		t.Fatalf("attempts=%d closed=%v", attempts, body.closed)
	}
}

type retryNetTimeout struct{}

func (retryNetTimeout) Error() string   { return "network timeout" }
func (retryNetTimeout) Timeout() bool   { return true }
func (retryNetTimeout) Temporary() bool { return true }

func TestRetryRecognizesNetworkTimeout(t *testing.T) {
	c := mustNewTestClient(t)
	attempts := 0
	c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return nil, retryNetTimeout{}
		}
		return retryResponse(200, `{"code":20000,"data":{"list":[]}}`), nil
	})
	if _, _, _, err := fetchPostsV3(c, 1, 20, 2, false, false); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d, want 2", attempts)
	}
}

func TestCommentRetryStopsOnExpiredCredentials(t *testing.T) {
	c := mustNewTestClient(t)
	attempts := 0
	c.GetHttpClient().Transport = crawlTransport(func(r *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return nil, context.DeadlineExceeded
		}
		return retryResponse(200, `{"code":40001,"message":"credential expired"}`), nil
	})
	comments, err := fetchAllComments(c, 10, 2)
	if err == nil || len(comments) != 0 {
		t.Fatalf("comments=%+v err=%v", comments, err)
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d, want 2", attempts)
	}
}
