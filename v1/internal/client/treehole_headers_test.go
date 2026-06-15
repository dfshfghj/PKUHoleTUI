package client

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
)

type captureRoundTripper struct {
	last *http.Request
	body string
}

func (c *captureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	c.last = req.Clone(req.Context())
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(c.body)),
		Request:    req,
	}, nil
}

func TestApplyTreeholeHeadersSetsUUIDAuthorizationAndXSRF(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	c := &Client{
		httpClient:    &http.Client{Jar: jar},
		authorization: "token-123",
		deviceUUID:    "uuid-456",
	}
	jar.SetCookies(&url.URL{Scheme: "https", Host: "treehole.pku.edu.cn"}, []*http.Cookie{{Name: "XSRF-TOKEN", Value: "xsrf-789"}})
	req, err := http.NewRequest(http.MethodGet, "https://treehole.pku.edu.cn/api/pku_hole", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	c.applyTreeholeHeaders(req)

	if got := req.Header.Get("uuid"); got != "uuid-456" {
		t.Fatalf("uuid = %q, want %q", got, "uuid-456")
	}
	if got := req.Header.Get("Authorization"); got != "Bearer token-123" {
		t.Fatalf("Authorization = %q, want bearer token", got)
	}
	if got := req.Header.Get("x-xsrf-token"); got != "xsrf-789" {
		t.Fatalf("x-xsrf-token = %q, want %q", got, "xsrf-789")
	}
}

func TestUnReadUsesUnifiedTreeholeHeaders(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	capture := &captureRoundTripper{body: `{"success":true}`}
	c := &Client{
		httpClient:    &http.Client{Jar: jar, Transport: capture},
		authorization: "token-123",
		deviceUUID:    "uuid-456",
	}
	jar.SetCookies(&url.URL{Scheme: "https", Host: "treehole.pku.edu.cn"}, []*http.Cookie{{Name: "XSRF-TOKEN", Value: "xsrf-789"}})

	resp, err := c.UnRead()
	if err != nil {
		t.Fatalf("UnRead: %v", err)
	}
	resp.Body.Close()

	if capture.last == nil {
		t.Fatal("expected request capture")
	}
	if got := capture.last.Header.Get("uuid"); got != "uuid-456" {
		t.Fatalf("uuid = %q, want %q", got, "uuid-456")
	}
	if got := capture.last.Header.Get("Authorization"); got != "Bearer token-123" {
		t.Fatalf("Authorization = %q, want bearer token", got)
	}
	if got := capture.last.Header.Get("x-xsrf-token"); got != "xsrf-789" {
		t.Fatalf("x-xsrf-token = %q, want %q", got, "xsrf-789")
	}
}

func TestProbeSessionTreatsHTMLResponseAsLoginUnavailable(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	c := &Client{
		httpClient: &http.Client{Jar: jar, Transport: &captureRoundTripper{body: "<html>login</html>"}},
	}

	status := c.ProbeSession()

	if status.FailureKind != SessionFailureLogin {
		t.Fatalf("failure kind = %q, want %q", status.FailureKind, SessionFailureLogin)
	}
	if status.Message != "登录态不可用" {
		t.Fatalf("message = %q, want 登录态不可用", status.Message)
	}
}
