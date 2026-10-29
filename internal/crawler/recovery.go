package crawler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"treehole/internal/client"
	"treehole/internal/db"
	"treehole/internal/models"
)

const maxRecoveryPages = 32
const maxPageReductions = 3

// recoveryDumpDir 是容错失败时原始响应的落盘目录；测试会覆盖为临时目录。
var recoveryDumpDir = "."

func isServerTimeout(err error) bool {
	var status *client.HTTPStatusError
	return errors.As(err, &status) && (status.StatusCode == http.StatusRequestTimeout || status.StatusCode == http.StatusGatewayTimeout)
}

func fetchPostListPage(c *client.Client, page, limit, commentLimit int) (map[string]interface{}, []byte, error) {
	body, err := retryTimeout(c, fmt.Sprintf("posts page=%d limit=%d", page, limit), func(attempt *client.Client) ([]byte, error) {
		resp, err := attempt.GetPostsList(page, limit, commentLimit, 1)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, &client.HTTPStatusError{StatusCode: resp.StatusCode}
		}
		return io.ReadAll(resp.Body)
	})
	if err != nil {
		return nil, nil, err
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, body, err
	}
	if code, ok := raw["code"].(float64); !ok || code != 20000 {
		return nil, body, fmt.Errorf("API error: code=%v message=%v", raw["code"], raw["message"])
	}
	data, ok := raw["data"].(map[string]interface{})
	if !ok {
		return nil, body, fmt.Errorf("invalid posts data")
	}
	if _, ok := data["list"].([]interface{}); !ok {
		return nil, body, fmt.Errorf("invalid posts list")
	}
	return raw, nil, nil
}

type recoveryDump struct {
	GeneratedAt    string                   `json:"generated_at"`
	Page           int                      `json:"page"`
	Limit          int                      `json:"limit"`
	Error          string                   `json:"error"`
	FailedResponse string                   `json:"failed_response,omitempty"`
	Responses      []map[string]interface{} `json:"responses"`
}

func writeRecoveryDump(page, limit int, cause error, responses []map[string]interface{}, failedBody []byte) error {
	dump := recoveryDump{
		GeneratedAt:    time.Now().Format(time.RFC3339),
		Page:           page,
		Limit:          limit,
		Error:          cause.Error(),
		FailedResponse: string(failedBody),
		Responses:      responses,
	}
	data, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化容错失败原始响应失败: %w", err)
	}
	if err := os.MkdirAll(recoveryDumpDir, 0755); err != nil {
		return err
	}
	filename := filepath.Join(recoveryDumpDir, fmt.Sprintf("recovery_failure_%s.json", time.Now().Format("2006-01-02_15-04-05.000")))
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return err
	}
	log.Printf("[Crawler] 容错失败原始响应已保存: %s", filename)
	return nil
}

// fetchPostListRecovering covers the original half-open offset window with
// smaller pages. Floor division at both ends intentionally includes partial
// boundary pages; trimming happens only after those pages have been fetched.
func fetchPostListRecovering(c *client.Client, database *db.Database, page, limit, commentLimit int, previous []models.Post) (raw map[string]interface{}, err error) {
	if page <= 0 || limit <= 0 {
		return nil, fmt.Errorf("post page and limit must be positive")
	}
	var responses []map[string]interface{}
	var failedBody []byte
	// 保留失败前已抓到的分页响应，便于排查服务端超时/异常返回。
	defer func() {
		if err == nil || len(responses) == 0 && len(failedBody) == 0 {
			return
		}
		if dumpErr := writeRecoveryDump(page, limit, err, responses, failedBody); dumpErr != nil {
			log.Printf("[Crawler] 保存容错失败原始响应失败: %v", dumpErr)
		}
	}()
	raw, failedBody, err = fetchPostListPage(c, page, limit, commentLimit)
	if err == nil || !isServerTimeout(err) || limit == 1 {
		return raw, err
	}
	log.Printf("[Crawler] 服务端超时，拆分 posts page=%d limit=%d", page, limit)
	size := limit / 2
	depth := 1
	start, end := (int64(page)-1)*int64(limit), int64(page)*int64(limit)
	budget := maxRecoveryPages - 1
	var total *int64
	fetch := func(p, n int) ([]interface{}, error) {
		if budget == 0 {
			return nil, fmt.Errorf("post recovery exceeded %d page requests", maxRecoveryPages)
		}
		budget--
		r, body, e := fetchPostListPage(c, p, n, commentLimit)
		if e != nil {
			if len(body) > 0 {
				failedBody = body
			}
			return nil, e
		}
		responses = append(responses, r)
		data := r["data"].(map[string]interface{})
		items := data["list"].([]interface{})
		if v, ok := data["total"].(float64); ok {
			count := int64(v)
			if count < 0 || v != float64(count) {
				return nil, fmt.Errorf("invalid post total")
			}
			if total != nil && *total != count {
				return nil, fmt.Errorf("post total changed during recovery")
			}
			total = &count
		}
		offset := (int64(p) - 1) * int64(n)
		want := int64(n)
		if total != nil && offset+want > *total {
			want = max(int64(0), *total-offset)
		}
		if int64(len(items)) != want {
			return nil, fmt.Errorf("incomplete recovery page %d: got %d want %d", p, len(items), want)
		}
		return items, nil
	}
	// Include the last successful PID as an overlap anchor. Density supplies a
	// search seed only; the locator verifies its actual page before use.
	var anchor int32
	for _, post := range previous {
		if !post.IsTop && (anchor == 0 || post.Pid < anchor) {
			anchor = post.Pid
		}
	}
	if anchor > 0 {
		var latest int32
		for headPage := 1; latest == 0; headPage++ {
			newest, e := fetch(headPage, 1)
			if e != nil {
				return nil, e
			}
			if len(newest) == 0 {
				return nil, fmt.Errorf("cannot locate recovery anchor without newest PID")
			}
			for _, item := range newest {
				b, _ := json.Marshal(item)
				var post TempPost
				if e := json.Unmarshal(b, &post); e != nil {
					return nil, e
				}
				if !post.IsTop && post.Pid > latest {
					latest = post.Pid
				}
			}
		}
		sample := db.PIDSample{}
		if database != nil {
			var e error
			sample, e = database.GetPostPIDSample(anchor, latest)
			if e != nil {
				return nil, e
			}
		}
		var located int
		for {
			var e error
			located, e = locatePostPage(anchor, latest, size, sample, func(p, n int) ([]models.Post, error) {
				items, err := fetch(p, n)
				if err != nil {
					return nil, err
				}
				posts := make([]models.Post, 0, len(items))
				for _, item := range items {
					b, err := json.Marshal(item)
					if err != nil {
						return nil, err
					}
					var post TempPost
					if err := json.Unmarshal(b, &post); err != nil {
						return nil, err
					}
					posts = append(posts, models.Post{Pid: post.Pid, IsTop: post.IsTop})
				}
				return posts, nil
			})
			if e == nil {
				break
			}
			if !isServerTimeout(e) || size <= 1 || depth >= maxPageReductions {
				return nil, e
			}
			size /= 2
			depth++
		}
		// The preceding page also covers a deleted anchor lying between two pages.
		start = min(start, int64(max(1, located-1)-1)*int64(size))
		end = max(end, int64(located)*int64(size)+int64(limit))
	}
	var window func(int64, int64, int, int) ([]interface{}, error)
	window = func(lo, hi int64, n, depth int) ([]interface{}, error) {
		var result []interface{}
		for p := lo/int64(n) + 1; p <= (hi-1)/int64(n)+1; p++ {
			from := max(lo, (p-1)*int64(n))
			to := min(hi, p*int64(n))
			items, e := fetch(int(p), n)
			if e != nil {
				if !isServerTimeout(e) || depth >= maxPageReductions || n <= 1 {
					return nil, e
				}
				log.Printf("[Crawler] 缩小恢复页 page=%d limit=%d -> %d", p, n, n/2)
				part, e := window(from, to, n/2, depth+1)
				if e != nil {
					return nil, e
				}
				result = append(result, part...)
				continue
			}
			if total != nil {
				to = min(to, *total)
			}
			if from >= to {
				break
			}
			a, b := from-(p-1)*int64(n), to-(p-1)*int64(n)
			if b > int64(len(items)) {
				return nil, fmt.Errorf("recovery did not cover offset %d", to)
			}
			result = append(result, items[a:b]...)
		}
		return result, nil
	}
	items, err := window(start, end, size, depth)
	if err != nil {
		return nil, err
	}
	seen := make(map[int32]bool)
	var last int32
	hasOlder := false
	for _, item := range items {
		b, _ := json.Marshal(item)
		var post TempPost
		if err := json.Unmarshal(b, &post); err != nil {
			return nil, err
		}
		if post.Pid <= 0 || seen[post.Pid] || (!bool(post.IsTop) && last > 0 && post.Pid >= last) {
			return nil, fmt.Errorf("post recovery has overlapping or unordered PID boundaries")
		}
		seen[post.Pid] = true
		if !post.IsTop {
			last = post.Pid
			if post.Pid < anchor {
				hasOlder = true
			}
		}
	}
	if anchor > 0 && !hasOlder {
		items = nil
	}

	return recoveredPostList(items, total, responses), nil
}

func recoveredPostList(items []interface{}, total *int64, responses []map[string]interface{}) map[string]interface{} {
	if items == nil {
		items = []interface{}{}
	}
	data := map[string]interface{}{"list": items}
	if total != nil {
		data["total"] = float64(*total)
	}
	return map[string]interface{}{"code": float64(20000), "data": data, "recovery_pages": responses}
}
