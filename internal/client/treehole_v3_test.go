package client

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestFlattenTagsSupportsTagNameChildren(t *testing.T) {
	data := []byte(`{
		"data": {
			"list": [
				{
					"id": 22,
					"tag_name": "课程学业",
					"children": [
						{"id": 1, "tag_name": "课程心得", "type_id": 22}
					]
				}
			]
		}
	}`)
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

func TestCreatePostV3SerializesImagePayload(t *testing.T) {
	capture := newV3CaptureClient(t, `{"code":20000,"data":{"pid":1,"text":"ok","timestamp":1}}`)

	if _, err := capture.client.CreatePostV3(CreatePostPayload{Type: "image", Text: "hello", MediaIDs: "35802,35803"}); err != nil {
		t.Fatalf("CreatePostV3: %v", err)
	}
	assertJSONField(t, capture.rt.body, "type", "image")
	assertJSONField(t, capture.rt.body, "text", "hello")
	assertJSONField(t, capture.rt.body, "media_ids", "35802,35803")
}

func TestCreateCommentV3SerializesMediaIDs(t *testing.T) {
	capture := newV3CaptureClient(t, `{"code":20000,"data":{"cid":1,"pid":99,"text":"ok","timestamp":1}}`)

	if _, err := capture.client.CreateCommentV3(CreateCommentPayload{PID: 99, Text: "hello", MediaIDs: "35802,35803"}); err != nil {
		t.Fatalf("CreateCommentV3: %v", err)
	}
	assertJSONField(t, capture.rt.body, "pid", float64(99))
	assertJSONField(t, capture.rt.body, "text", "hello")
	assertJSONField(t, capture.rt.body, "media_ids", "35802,35803")
}

func TestUploadImageV3SendsMultipartFile(t *testing.T) {
	capture := newV3CaptureClient(t, `{"code":20000,"data":{"id":35803,"url":"x.jpg"}}`)
	file, err := os.CreateTemp(t.TempDir(), "upload-*.jpg")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := file.WriteString("image-bytes"); err != nil {
		t.Fatalf("write temp image: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close temp image: %v", err)
	}

	id, err := capture.client.UploadImageV3(file.Name())
	if err != nil {
		t.Fatalf("UploadImageV3: %v", err)
	}
	if id != "35803" {
		t.Fatalf("media id = %q, want 35803", id)
	}
	if capture.rt.last.URL.Path != "/chapi/api/v3/media/uploadImage" {
		t.Fatalf("upload path = %q", capture.rt.last.URL.Path)
	}
	mediaType, params, err := mime.ParseMediaType(capture.rt.last.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("ParseMediaType: %v", err)
	}
	if mediaType != "multipart/form-data" {
		t.Fatalf("content type = %q, want multipart/form-data", mediaType)
	}
	reader := multipart.NewReader(bytes.NewReader(capture.rt.body.Bytes()), params["boundary"])
	part, err := reader.NextPart()
	if err != nil {
		t.Fatalf("NextPart: %v", err)
	}
	if part.FormName() != "file" {
		t.Fatalf("form name = %q, want file", part.FormName())
	}
	if part.FileName() == "" {
		t.Fatal("file name should be set")
	}
	data, err := io.ReadAll(part)
	if err != nil {
		t.Fatalf("ReadAll part: %v", err)
	}
	if string(data) != "image-bytes" {
		t.Fatalf("uploaded bytes = %q", string(data))
	}
}

func TestGetCourseTableV2CleansCourseNames(t *testing.T) {
	capture := newV3CaptureClient(t, `{"code":20000,"data":{"course":[{"timeNum":"第一节","mon":{"courseName":"<font><b>量子力学(主)<br>上课信息：1-15周<br>考试信息：x</b></font>","parity":"","sty":""},"tue":{"courseName":"","parity":"","sty":""},"wed":{"courseName":"","parity":"","sty":""},"thu":{"courseName":"","parity":"","sty":""},"fri":{"courseName":"","parity":"","sty":""},"sat":{"courseName":"","parity":"","sty":""},"sun":{"courseName":"","parity":"","sty":""}}]}}`)

	rows, err := capture.client.GetCourseTableV2()
	if err != nil {
		t.Fatalf("GetCourseTableV2: %v", err)
	}
	if capture.rt.last.URL.Path != "/chapi/api/getCoursetable_v2" {
		t.Fatalf("course table path = %q", capture.rt.last.URL.Path)
	}
	if len(rows) != 1 || rows[0].TimeNum != "1" || rows[0].Mon.CourseName != "量子力学(主)" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestGetCourseScoresV2ParsesSummaryAndCourses(t *testing.T) {
	capture := newV3CaptureClient(t, `{"code":20000,"data":{"score":{"cjxx":[{"xndxqpx":"2025-261","kcmc":"统计力学","xf":"3","xqcj":"96","kclbmc":"专业必修"}],"gpaHM":{"gpa":"3.892","zxf":"93.0","xxxf":"93","xkms":"36"},"gpa":{"gpa":"3.892","xxxf":"93"}},"gpa":{"data":[{"xndxq":"25-26-1","gpa":"3.925"}]}}}`)

	summary, err := capture.client.GetCourseScoresV2()
	if err != nil {
		t.Fatalf("GetCourseScoresV2: %v", err)
	}
	if capture.rt.last.URL.Path != "/chapi/api/course/score_v2" {
		t.Fatalf("score path = %q", capture.rt.last.URL.Path)
	}
	if summary.GPA != "3.892" || summary.TotalCredit != "93.0" || len(summary.Scores) != 1 || summary.Scores[0].Name != "统计力学" {
		t.Fatalf("summary = %+v", summary)
	}
	if len(summary.GPATerms) != 1 || summary.GPATerms[0].GPA != "3.925" {
		t.Fatalf("gpa terms = %+v", summary.GPATerms)
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
