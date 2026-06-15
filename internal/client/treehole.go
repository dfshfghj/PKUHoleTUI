package client

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"treehole/internal/config"
)

type TreeHoleWeb string

const (
	OAUTH_LOGIN       TreeHoleWeb = "https://iaaa.pku.edu.cn/iaaa/oauthlogin.do"
	REDIR_URL         TreeHoleWeb = "https://treehole.pku.edu.cn/cas_iaaa_login?uuid=fc71db5799cf&plat=web"
	SSO_LOGIN         TreeHoleWeb = "http://treehole.pku.edu.cn/cas_iaaa_login"
	UN_READ           TreeHoleWeb = "https://treehole.pku.edu.cn/api/mail/un_read"
	SEARCH            TreeHoleWeb = "https://treehole.pku.edu.cn/api/pku_hole"
	COMMENT           TreeHoleWeb = "https://treehole.pku.edu.cn/api/pku_comment_v3"
	FOLLOW            TreeHoleWeb = "https://treehole.pku.edu.cn/api/pku_attention"
	GET_FOLLOW        TreeHoleWeb = "https://treehole.pku.edu.cn/api/follow_v2"
	REPORT            TreeHoleWeb = "https://treehole.pku.edu.cn/api/pku_comment/report"
	LOGIN_BY_TOKEN    TreeHoleWeb = "https://treehole.pku.edu.cn/api/login_iaaa_check_token"
	LOGIN_BY_MESSAGE  TreeHoleWeb = "https://treehole.pku.edu.cn/api/jwt_msg_verify"
	SEND_MESSAGE      TreeHoleWeb = "https://treehole.pku.edu.cn/api/jwt_send_msg"
	COURSE_TABLE      TreeHoleWeb = "https://treehole.pku.edu.cn/api/getCoursetable_v2"
	GRADE             TreeHoleWeb = "https://treehole.pku.edu.cn/api/course/score_v2"
	NEW_POSTS_LIST    TreeHoleWeb = "https://treehole.pku.edu.cn/chapi/api/v3/hole/list_comments"
	NEW_COMMENTS_LIST TreeHoleWeb = "https://treehole.pku.edu.cn/chapi/api/v3/comment/list"
)

type Client struct {
	httpClient    *http.Client
	authorization string
	deviceUUID    string
}

func NewClient(deviceUUID string) (*Client, error) {
	if deviceUUID == "" {
		deviceUUID = "Web_PKUHOLE_2.0.0_WEB_UUID_00000000-0000-0000-0000-000000000000"
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Jar: jar,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 300 * time.Second,
	}

	c := &Client{
		httpClient: client,
		deviceUUID: deviceUUID,
	}

	c.httpClient.Transport = &userAgentRoundTripper{
		transport: c.httpClient.Transport,
		userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36 Edg/146.0.0.0",
	}

	err = c.LoadCookies()
	if err != nil {
		log.Printf("加载cookies失败: %v\n", err)
	}

	if token := c.GetPkuToken(); token != "" {
		c.authorization = token
	}

	return c, nil
}

// userAgentRoundTripper 用于设置User-Agent的中间件
type userAgentRoundTripper struct {
	transport http.RoundTripper
	userAgent string
}

func (u *userAgentRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", u.userAgent)
	if u.transport == nil {
		return http.DefaultTransport.RoundTrip(req)
	}
	return u.transport.RoundTrip(req)
}

func (c *Client) GetPkuToken() string {
	cookies := c.httpClient.Jar.Cookies(&url.URL{Scheme: "https", Host: "treehole.pku.edu.cn"})
	for _, cookie := range cookies {
		if cookie.Name == "pku_token" {
			return cookie.Value
		}
	}
	return ""
}

func (c *Client) GetXSRFToken() string {
	cookies := c.httpClient.Jar.Cookies(&url.URL{Scheme: "https", Host: "treehole.pku.edu.cn"})
	for _, cookie := range cookies {
		if cookie.Name == "XSRF-TOKEN" {
			return cookie.Value
		}
	}
	return ""
}

func (c *Client) SetPkuToken(token string) {
	cookie := &http.Cookie{
		Name:   "pku_token",
		Value:  token,
		Domain: "treehole.pku.edu.cn",
		Path:   "/",
	}
	c.httpClient.Jar.SetCookies(&url.URL{Scheme: "https", Host: "treehole.pku.edu.cn"}, []*http.Cookie{cookie})
}

func (c *Client) applyTreeholeHeaders(req *http.Request) {
	if req == nil {
		return
	}
	if c.deviceUUID != "" {
		req.Header.Set("uuid", c.deviceUUID)
	}
	if c.authorization != "" {
		req.Header.Set("Authorization", "Bearer "+c.authorization)
	}
	if token := c.GetXSRFToken(); token != "" {
		req.Header.Set("x-xsrf-token", token)
	}
}

func (c *Client) OAuthLogin(username, password string) (map[string]interface{}, error) {
	data := url.Values{}
	data.Set("appid", "PKU Helper")
	data.Set("userName", username)
	data.Set("password", password)
	data.Set("randCode", "")
	data.Set("smsCode", "")
	data.Set("otpCode", "")
	data.Set("redirUrl", string(REDIR_URL))

	resp, err := c.httpClient.PostForm(string(OAUTH_LOGIN), data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth login failed with status: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) SSOLogin(token string) error {
	params := url.Values{}
	params.Set("uuid", generateUUID())
	params.Set("plat", "web")
	params.Set("_rand", fmt.Sprintf("%f", randFloat()))
	params.Set("token", token)

	reqURL := string(SSO_LOGIN) + "?" + params.Encode()
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return err
	}
	c.applyTreeholeHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sso login failed with status: %d", resp.StatusCode)
	}

	re := regexp.MustCompile(`token=(.*)`)
	matches := re.FindStringSubmatch(resp.Request.URL.String())
	if len(matches) < 2 {
		return fmt.Errorf("failed to extract token from redirect URL")
	}

	c.authorization = matches[1]
	c.SetPkuToken(c.authorization)

	return nil
}

func (c *Client) UnRead() (*http.Response, error) {
	req, err := http.NewRequest("GET", string(UN_READ), nil)
	if err != nil {
		return nil, err
	}

	c.applyTreeholeHeaders(req)

	return c.httpClient.Do(req)
}

func (c *Client) LoginByToken(token string) (*http.Response, error) {
	data := url.Values{}
	data.Set("code", token)

	req, err := http.NewRequest("POST", string(LOGIN_BY_TOKEN), strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.applyTreeholeHeaders(req)

	return c.httpClient.Do(req)
}

func (c *Client) LoginByMessage(code string) (*http.Response, error) {
	data := url.Values{}
	data.Set("valid_code", code)

	req, err := http.NewRequest("POST", string(LOGIN_BY_MESSAGE), strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.applyTreeholeHeaders(req)

	return c.httpClient.Do(req)
}

func (c *Client) SendMessage() (*http.Response, error) {
	req, err := http.NewRequest("POST", string(SEND_MESSAGE), nil)
	if err != nil {
		return nil, err
	}
	c.applyTreeholeHeaders(req)

	return c.httpClient.Do(req)
}

func (c *Client) GetPost(postID int) (map[string]interface{}, error) {
	reqURL := fmt.Sprintf("https://treehole.pku.edu.cn/api/pku/%d", postID)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	c.applyTreeholeHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get post failed with status: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) GetComment(postID, page, limit int, sort string) (map[string]interface{}, error) {
	postURL := fmt.Sprintf("https://treehole.pku.edu.cn/api/pku_comment_v3/%d", postID)
	params := url.Values{}
	params.Set("page", strconv.Itoa(page))
	params.Set("limit", strconv.Itoa(limit))
	params.Set("sort", sort)

	reqURL := postURL + "?" + params.Encode()
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	c.applyTreeholeHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get comment failed with status: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) Search(keyword string, page, limit int, label interface{}) (*http.Response, error) {
	params := url.Values{}
	params.Set("page", strconv.Itoa(page))
	params.Set("limit", strconv.Itoa(limit))
	if keyword != "" {
		params.Set("keyword", keyword)
	}
	if label != nil {
		params.Set("label", fmt.Sprintf("%v", label))
	}

	reqURL := string(SEARCH) + "?" + params.Encode()
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	c.applyTreeholeHeaders(req)

	return c.httpClient.Do(req)
}

func (c *Client) Follow(postID int) (*http.Response, error) {
	reqURL := fmt.Sprintf("%s/%d", FOLLOW, postID)
	req, err := http.NewRequest("POST", reqURL, nil)
	if err != nil {
		return nil, err
	}
	c.applyTreeholeHeaders(req)

	return c.httpClient.Do(req)
}

func (c *Client) GetFollow(page, limit int) (*http.Response, error) {
	params := url.Values{}
	params.Set("page", strconv.Itoa(page))
	params.Set("limit", strconv.Itoa(limit))

	reqURL := string(GET_FOLLOW) + "?" + params.Encode()
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	c.applyTreeholeHeaders(req)

	return c.httpClient.Do(req)
}

func (c *Client) Comment(postID int, text string, commentID *int) (*http.Response, error) {
	data := url.Values{}
	data.Set("pid", strconv.Itoa(postID))
	data.Set("text", text)
	if commentID != nil {
		data.Set("comment_id", strconv.Itoa(*commentID))
	}

	req, err := http.NewRequest("POST", string(COMMENT), strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.applyTreeholeHeaders(req)

	return c.httpClient.Do(req)
}

func (c *Client) Report(tp string, xid int, other, reason string) (*http.Response, error) {
	var reqURL string
	var data url.Values

	if tp == "post" {
		reqURL = fmt.Sprintf("%s/%d", REPORT, xid)
		data = url.Values{
			"other":  {other},
			"reason": {reason},
		}
	} else if tp == "comment" {
		reqURL = string(REPORT)
		data = url.Values{
			"cid":    {strconv.Itoa(xid)},
			"other":  {other},
			"reason": {reason},
		}
	} else {
		return nil, fmt.Errorf("invalid report type: %s", tp)
	}

	req, err := http.NewRequest("POST", reqURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.applyTreeholeHeaders(req)

	return c.httpClient.Do(req)
}

func (c *Client) GetCourseTable() (*http.Response, error) {
	req, err := http.NewRequest("GET", string(COURSE_TABLE), nil)
	if err != nil {
		return nil, err
	}
	c.applyTreeholeHeaders(req)

	return c.httpClient.Do(req)
}

func (c *Client) GetGrade() (*http.Response, error) {
	req, err := http.NewRequest("GET", string(GRADE), nil)
	if err != nil {
		return nil, err
	}
	c.applyTreeholeHeaders(req)

	return c.httpClient.Do(req)
}

func (c *Client) SaveCookies() error {
	if err := config.EnsureRuntimeFiles(); err != nil {
		return err
	}
	cookies := c.httpClient.Jar.Cookies(&url.URL{Scheme: "https", Host: "treehole.pku.edu.cn"})

	var cookiesList []map[string]interface{}
	for _, cookie := range cookies {
		cookieMap := map[string]interface{}{
			"name":   cookie.Name,
			"value":  cookie.Value,
			"domain": cookie.Domain,
			"path":   cookie.Path,
			"secure": cookie.Secure,
		}
		if !cookie.Expires.IsZero() {
			cookieMap["expires"] = cookie.Expires.Unix()
		}
		cookiesList = append(cookiesList, cookieMap)
	}

	cookiePath, err := config.CookiesPath()
	if err != nil {
		return err
	}

	file, err := os.Create(cookiePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ")
	return encoder.Encode(cookiesList)
}

func (c *Client) LoadCookies() error {
	if err := config.EnsureRuntimeFiles(); err != nil {
		return err
	}
	cookiePath, err := config.CookiesPath()
	if err != nil {
		return err
	}

	file, err := os.Open(cookiePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var cookiesList []map[string]interface{}
	err = json.NewDecoder(file).Decode(&cookiesList)
	if err != nil {
		return err
	}

	var cookies []*http.Cookie
	for _, cookieMap := range cookiesList {
		cookie := &http.Cookie{
			Name:   cookieMap["name"].(string),
			Value:  cookieMap["value"].(string),
			Domain: cookieMap["domain"].(string),
			Path:   cookieMap["path"].(string),
			Secure: cookieMap["secure"].(bool),
		}
		if expires, ok := cookieMap["expires"]; ok {
			if exp, ok := expires.(float64); ok {
				cookie.Expires = time.Unix(int64(exp), 0)
			}
		}
		cookies = append(cookies, cookie)
	}

	c.httpClient.Jar.SetCookies(&url.URL{Scheme: "https", Host: "treehole.pku.edu.cn"}, cookies)
	return nil
}

func (c *Client) GetPostsList(page, limit, commentLimit, commentStream int) (*http.Response, error) {
	params := url.Values{}
	params.Set("page", strconv.Itoa(page))
	params.Set("limit", strconv.Itoa(limit))
	params.Set("comment_limit", strconv.Itoa(commentLimit))
	params.Set("comment_stream", strconv.Itoa(commentStream))

	reqURL := string(NEW_POSTS_LIST) + "?" + params.Encode()
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	c.applyTreeholeHeaders(req)

	return c.httpClient.Do(req)
}

func (c *Client) GetCommentsByPid(pid, page, limit, sort, commentStream int) (*http.Response, error) {
	params := url.Values{}
	params.Set("pid", strconv.Itoa(pid))
	params.Set("page", strconv.Itoa(page))
	params.Set("limit", strconv.Itoa(limit))
	params.Set("sort", strconv.Itoa(sort))
	params.Set("comment_stream", strconv.Itoa(commentStream))

	reqURL := string(NEW_COMMENTS_LIST) + "?" + params.Encode()
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	c.applyTreeholeHeaders(req)

	return c.httpClient.Do(req)
}

func (c *Client) GetHttpClient() *http.Client {
	return c.httpClient
}

func (c *Client) GetAuthorization() string {
	return c.authorization
}

func randFloat() float64 {
	return rand.Float64()
}

func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b[12:16])
}
