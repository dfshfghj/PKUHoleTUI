package handles

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"treehole/internal/db"
	"treehole/internal/models"
	"treehole/server/utils"

	"github.com/gin-gonic/gin"
)

func Health(c *gin.Context) {
	utils.RespondSuccess(c, gin.H{"status": "ok", "message": "PKU Hole API is running"})
}

func Help(c *gin.Context) {
	utils.RespondSuccess(c, []gin.H{
		{"method": "GET", "route": "/health", "description": "Check whether the API server is running."},
		{"method": "GET", "route": "/help", "description": "List available routes with brief English descriptions."},
		{"method": "GET", "route": "/posts?begin=0&limit=25&keyword=&order_by=&begin_time=&end_time=", "description": "List posts with optional cursor pagination, keyword search, ordering, and unix-second time range filtering (begin_time/end_time, inclusive)."},
		{"method": "GET", "route": "/post/:pid", "description": "Fetch a single post by post ID."},
		{"method": "GET", "route": "/comment?cid=123", "description": "Fetch a single comment by comment ID."},
		{"method": "GET", "route": "/comments/:pid?begin=0&limit=25&sort=0", "description": "List comments for a post with optional cursor pagination and sort order."},
		{"method": "GET", "route": "/media/image?id=123 or /media/image?pid=456", "description": "Serve a local image by media ID or post ID."},
		{"method": "GET", "route": "/chapi/api/v3/hole/list_comments?page=1&limit=10&comment_limit=10&keyword=&kind=&label=", "description": "Origin-compatible read-only mirror: list holes with embedded first-page comments."},
		{"method": "GET", "route": "/chapi/api/v3/hole/one?pid=123", "description": "Origin-compatible read-only mirror: single hole with comments."},
		{"method": "GET", "route": "/chapi/api/v3/hole/get?pid=123", "description": "Origin-compatible read-only mirror: single hole info."},
		{"method": "GET", "route": "/chapi/api/v3/comment/list?pid=123&page=1&limit=10&sort=0", "description": "Origin-compatible read-only mirror: paginated comments for a hole."},
		{"method": "GET", "route": "/chapi/api/v3/message/un_read | /chapi/api/v3/message/index | /chapi/api/v3/tags/tree", "description": "Origin-compatible read-only mirror: static defaults (no user state)."},
		{"method": "GET", "route": "/chapi/api/v3/media/getThumbnail | /chapi/api/v3/media/getMediaBinary | /chapi/api/v3/media/getImageBinary?id= or pid=", "description": "Origin-compatible read-only mirror: serve local images."},
	})
}

func GetPost(database *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		pid, err := strconv.ParseInt(c.Param("pid"), 10, 32)
		if err != nil {
			utils.RespondError(c, 400, "InvalidParam", err)
			return
		}

		p, err := database.GetPostByPid(int32(pid))
		if err != nil {
			utils.RespondError(c, 404, "NotFound", err)
			return
		}

		username := "anonymous"
		if !p.Anonymous {
			username = ""
		}

		postData := map[string]interface{}{
			"id":         p.Pid,
			"text":       p.Text,
			"userid":     65535,
			"username":   username,
			"timestamp":  p.Timestamp,
			"reply":      p.Reply,
			"likenum":    p.Likenum,
			"is_follow":  0,
			"praise_num": p.PraiseNum,
			"is_like":    0,
			"type":       p.Type,
			"tags":       []string{},
			"media_ids":  p.MediaIds,
		}

		utils.RespondSuccess(c, postData)
	}
}

func GetComment(database *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		cid, err := strconv.ParseInt(c.Query("cid"), 10, 32)
		if err != nil {
			utils.RespondError(c, 400, "InvalidParam", err)
			return
		}

		comment, err := database.GetCommentByCid(int32(cid))
		if err != nil {
			utils.RespondError(c, 404, "NotFound", err)
			return
		}

		utils.RespondSuccess(c, serializeComment(comment))
	}
}

// GetPosts 统一处理帖子获取和搜索
func GetPosts(database *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		keyword := c.Query("keyword")
		orderBy := c.Query("order_by")
		id := c.Query("id")

		limit, err := strconv.Atoi(c.DefaultQuery("limit", "25"))
		if err != nil || limit < 1 {
			limit = 25
		}
		if limit > 100 {
			limit = 100
		}

		cursor, _ := strconv.Atoi(c.DefaultQuery("begin", "0"))

		var timeRange *db.TimeRange
		beginTimeStr := c.Query("begin_time")
		endTimeStr := c.Query("end_time")
		if beginTimeStr != "" || endTimeStr != "" {
			var beginTS, endTS int64
			if beginTimeStr != "" {
				beginTS, err = strconv.ParseInt(beginTimeStr, 10, 64)
				if err != nil || beginTS < 0 {
					utils.RespondError(c, 400, "InvalidParam", errors.New("invalid begin_time, expect unix seconds"))
					return
				}
			}
			if endTimeStr != "" {
				endTS, err = strconv.ParseInt(endTimeStr, 10, 64)
				if err != nil || endTS < 0 {
					utils.RespondError(c, 400, "InvalidParam", errors.New("invalid end_time, expect unix seconds"))
					return
				}
			}
			if beginTS > 0 && endTS > 0 && beginTS > endTS {
				utils.RespondError(c, 400, "InvalidParam", errors.New("begin_time is after end_time"))
				return
			}
			timeRange = &db.TimeRange{Begin: beginTS, End: endTS}
		}

		var posts []models.Post
		var dbErr error

		if id != "" {
			var pid int64
			var post *models.Post
			pid, _ = strconv.ParseInt(id, 10, 32)
			post, dbErr = database.GetPostByPid(int32(pid))
			posts = []models.Post{*post}
		} else if orderBy != "" && keyword == "" {
			posts, dbErr = database.GetPostsOrderByRange(orderBy, timeRange, cursor, limit)
		} else if orderBy != "" && keyword != "" {
			posts, dbErr = database.SearchPostsOrderByRange(keyword, orderBy, timeRange, cursor, limit)
		} else if keyword != "" {
			posts, dbErr = database.SearchPostsCursorRange(keyword, timeRange, cursor, limit, false)
		} else {
			posts, dbErr = database.GetPostsCursorRange(timeRange, cursor, limit, false)
		}

		if dbErr != nil {
			utils.RespondError(c, 500, "ServerError", dbErr)
			return
		}

		postData := make([]map[string]interface{}, len(posts))
		for i, p := range posts {
			username := "anonymous"
			if !p.Anonymous {
				username = ""
			}

			postData[i] = map[string]interface{}{
				"id":         p.Pid,
				"text":       p.Text,
				"userid":     65535,
				"username":   username,
				"timestamp":  p.Timestamp,
				"reply":      p.Reply,
				"likenum":    p.Likenum,
				"is_follow":  0,
				"praise_num": p.PraiseNum,
				"is_like":    0,
				"type":       p.Type,
				"tags":       []string{},
				"media_ids":  p.MediaIds,
			}
		}

		utils.RespondSuccess(c, postData)
	}
}

func GetComments(database *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		pid, err := strconv.ParseInt(c.Param("pid"), 10, 32)
		if err != nil {
			utils.RespondError(c, 400, "InvalidParam", err)
			return
		}

		limit, err := strconv.Atoi(c.DefaultQuery("limit", "25"))
		if err != nil || limit < 1 {
			limit = 25
		}
		if limit > 100 {
			limit = 100
		}

		cursor, _ := strconv.ParseInt(c.DefaultQuery("begin", "0"), 10, 32)

		// 添加sort参数，0表示升序(asc)，1表示降序(desc)，默认为0（升序）
		sortParam := c.DefaultQuery("sort", "0")
		sortAsc := true
		if sortParam == "1" {
			sortAsc = false
		}

		comments, err := database.GetCommentsByPidCursor(int32(pid), int32(cursor), limit, sortAsc)
		if err != nil {
			utils.RespondError(c, 500, "ServerError", err)
			return
		}

		commentData := make([]map[string]interface{}, len(comments))
		for i, cmt := range comments {
			commentData[i] = serializeComment(&cmt)
		}

		utils.RespondSuccess(c, commentData)
	}
}

func serializeComment(cmt *models.Comment) map[string]interface{} {
	return map[string]interface{}{
		"cid":       cmt.Cid,
		"pid":       cmt.Pid,
		"userid":    65535,
		"username":  cmt.NameTag,
		"text":      cmt.Text,
		"timestamp": cmt.Timestamp,
		"quote":     cmt.Quote,
		"media_ids": cmt.MediaIds,
	}
}

func GetImage(c *gin.Context) {
	idStr := c.Query("id")
	pidStr := c.Query("pid")

	if idStr == "" && pidStr == "" {
		utils.RespondError(c, 400, "InvalidParam", errors.New("missing id or pid parameter"))
		return
	}

	var filename string
	dir := "data/images"

	if idStr != "" {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			utils.RespondError(c, 400, "InvalidParam", errors.New("invalid id"))
			return
		}

		filename = findImageFile(dir, strconv.Itoa(id))
	} else {
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			utils.RespondError(c, 400, "InvalidParam", errors.New("invalid pid"))
			return
		}
		filename = findImageFile(dir, strconv.Itoa(pid))
	}

	if filename == "" {
		c.Status(http.StatusNotFound)
		return
	}

	c.File(filename)
}

func findImageFile(dir, base string) string {
	exts := []string{".jpg", ".png", ".gif", ".webp"}
	for _, ext := range exts {
		path := filepath.Join(dir, base+ext)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
