package client

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFlattenTagsSupportsTagNameChildren(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(filename), "../..")
	data, err := os.ReadFile(filepath.Join(root, "doc", "tags.json"))
	if err != nil {
		t.Fatalf("read tags.json: %v", err)
	}
	var envelope struct {
		Data struct {
			List interface{} `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("unmarshal tags.json: %v", err)
	}
	tags := flattenTags(envelope.Data.List, 0)
	if len(tags) == 0 {
		t.Fatal("flattenTags returned no tags")
	}
	foundCategory := false
	foundChild := false
	for _, tag := range tags {
		if tag.ID == 22 && tag.Name == "课程学业" {
			foundCategory = true
		}
		if tag.ID == 1 && tag.Label == "课程心得" && tag.ParentID == 22 {
			foundChild = true
		}
	}
	if !foundCategory {
		t.Fatal("did not find top-level category tag")
	}
	if !foundChild {
		t.Fatal("did not find child tag parsed from tag_name/type_id")
	}
}

func TestListPostsV3IncludesPIDQuery(t *testing.T) {
	capture := newV3CaptureClient(t, `{"code":20000,"data":{"list":[],"total":0}}`)

	_, _, err := capture.client.ListPostsV3(V3ListPostsParams{
		Page:          2,
		Limit:         15,
		CommentLimit:  3,
		CommentStream: 1,
		Pid:           8123,
		Keyword:       "course",
		Label:         2,
	})
	if err != nil {
		t.Fatalf("ListPostsV3: %v", err)
	}
	if capture.rt.last == nil {
		t.Fatal("expected request capture")
	}
	query := capture.rt.last.URL.Query()
	if got := query.Get("pid"); got != "8123" {
		t.Fatalf("pid query = %q, want %q", got, "8123")
	}
	if got := query.Get("keyword"); got != "course" {
		t.Fatalf("keyword query = %q, want %q", got, "course")
	}
	if got := query.Get("page"); got != "2" {
		t.Fatalf("page query = %q, want %q", got, "2")
	}
}

func TestListPostsV3IncludesIsFollowQuery(t *testing.T) {
	capture := newV3CaptureClient(t, `{"code":20000,"data":{"list":[],"total":0}}`)
	isFollow := true

	_, _, err := capture.client.ListPostsV3(V3ListPostsParams{
		Page:     1,
		Limit:    20,
		IsFollow: &isFollow,
	})
	if err != nil {
		t.Fatalf("ListPostsV3: %v", err)
	}
	if capture.rt.last == nil {
		t.Fatal("expected request capture")
	}
	if got := capture.rt.last.URL.Query().Get("is_follow"); got != "1" {
		t.Fatalf("is_follow query = %q, want %q", got, "1")
	}
}

func TestCreateCommentV3WithQuoteSerializesCommentID(t *testing.T) {
	capture := newV3CaptureClient(t, `{"code":20000,"data":{"cid":1,"pid":99,"text":"ok","timestamp":1}}`)
	quoteID := int32(456)

	if _, err := capture.client.CreateCommentV3WithQuote(99, "hello", &quoteID); err != nil {
		t.Fatalf("CreateCommentV3WithQuote: %v", err)
	}
	assertJSONField(t, capture.rt.body, "pid", float64(99))
	assertJSONField(t, capture.rt.body, "text", "hello")
	assertJSONField(t, capture.rt.body, "comment_id", "456")
}

func TestCreateCommentV3WithoutQuoteOmitsCommentID(t *testing.T) {
	capture := newV3CaptureClient(t, `{"code":20000,"data":{"cid":1,"pid":99,"text":"ok","timestamp":1}}`)

	if _, err := capture.client.CreateCommentV3WithQuote(99, "hello", nil); err != nil {
		t.Fatalf("CreateCommentV3WithQuote: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(capture.rt.body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal request body: %v", err)
	}
	if _, ok := payload["comment_id"]; ok {
		t.Fatalf("comment_id should be omitted when quote is nil: %+v", payload)
	}
}

type v3CaptureClient struct {
	client *Client
	rt     *jsonCaptureRoundTripper
}

type jsonCaptureRoundTripper struct {
	last *http.Request
	body bytes.Buffer
	resp string
}

func (rt *jsonCaptureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.last = req.Clone(req.Context())
	rt.body.Reset()
	if req.Body != nil {
		defer req.Body.Close()
		if _, err := rt.body.ReadFrom(req.Body); err != nil {
			return nil, err
		}
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(rt.resp)),
		Request:    req,
	}, nil
}

func newV3CaptureClient(t *testing.T, response string) v3CaptureClient {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	rt := &jsonCaptureRoundTripper{resp: response}
	client := &Client{
		httpClient: &http.Client{Jar: jar, Transport: rt},
		deviceUUID: "test-uuid",
	}
	jar.SetCookies(&url.URL{Scheme: "https", Host: "treehole.pku.edu.cn"}, []*http.Cookie{{Name: "XSRF-TOKEN", Value: "xsrf-token"}})
	return v3CaptureClient{client: client, rt: rt}
}

func assertJSONField(t *testing.T, body bytes.Buffer, key string, want any) {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal request body: %v", err)
	}
	if got := payload[key]; got != want {
		t.Fatalf("%s = %#v, want %#v (payload=%+v)", key, got, want, payload)
	}
}
