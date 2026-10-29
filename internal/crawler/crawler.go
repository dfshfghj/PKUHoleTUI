package crawler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"treehole/internal/client"
	"treehole/internal/db"
	"treehole/internal/models"

	"github.com/chai2010/webp"
)

var (
	rawResponses   []map[string]interface{}
	rawResponsesMu sync.Mutex
	imageDir       = "data/images"
	thumbnailDir   = "data/thumbnails"
)

func ensureDir(dir string) {
	_ = os.MkdirAll(dir, 0755)
}

type APIResponse struct {
	Code int                    `json:"code"`
	Data map[string]interface{} `json:"data"`
}

// FetchResult holds the result of fetching a single page
type FetchResult struct {
	PostCount    int
	CommentCount int
	Posts        []models.Post
}

// FetchAndSave fetches one page of posts and saves them to the database
func FetchAndSave(c *client.Client, database *db.Database, page int, saveJSON bool, postLimit int, commentLimit int, fetchImages bool, convertWebp bool) (FetchResult, error) {
	return fetchAndSave(c, database, page, saveJSON, postLimit, commentLimit, fetchImages, convertWebp, nil)
}

func fetchAndSave(c *client.Client, database *db.Database, page int, saveJSON bool, postLimit int, commentLimit int, fetchImages bool, convertWebp bool, previousPagePosts []models.Post) (FetchResult, error) {
	var result FetchResult

	posts, comments, rawResponse, err := fetchPostsV3WithHistory(c, database, page, postLimit, commentLimit, fetchImages, convertWebp, previousPagePosts)
	if err != nil {
		return result, err
	}

	result.PostCount = len(posts)
	result.CommentCount = len(comments)
	result.Posts = posts

	missing, err := missingPostsInPage(database, posts, previousPagePosts)
	if err != nil {
		return result, err
	}
	if err := database.SaveCrawlSnapshot(posts, comments, missing); err != nil {
		return result, fmt.Errorf("写入抓取结果失败: %w", err)
	}

	// 如果启用JSON保存，将原始响应添加到全局列表
	if saveJSON && rawResponse != nil {
		rawResponsesMu.Lock()
		rawResponses = append(rawResponses, rawResponse)
		rawResponsesMu.Unlock()
		log.Printf("[Crawler] 原始响应已添加到内存缓存")
	}

	return result, nil
}

// RawResponses 返回当前存储的原始响应数量
func RawResponses() int {
	rawResponsesMu.Lock()
	defer rawResponsesMu.Unlock()
	return len(rawResponses)
}

// SaveRawResponsesToFile 将所有收集的原始响应保存到JSON文件
func SaveRawResponsesToFile() error {
	rawResponsesMu.Lock()
	if len(rawResponses) == 0 {
		rawResponsesMu.Unlock()
		return nil
	}
	responsesCopy := make([]map[string]interface{}, len(rawResponses))
	copy(responsesCopy, rawResponses)
	rawResponses = nil
	rawResponsesMu.Unlock()

	// 生成带时间戳的文件名
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("raw_responses_%s.json", timestamp)

	data, err := json.MarshalIndent(responsesCopy, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化原始响应失败: %w", err)
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("写入JSON文件失败: %w", err)
	}

	log.Printf("[Crawler] 原始响应已保存到文件: %s (%d 个响应)", filename, len(responsesCopy))

	return nil
}

// TempPost 用于匹配API响应的临时帖子结构体
type TempPost struct {
	Pid             int32          `json:"pid"`
	Text            string         `json:"text"`
	Type            string         `json:"type"`
	Timestamp       int32          `json:"timestamp"`
	Hidden          int            `json:"hidden"`
	Reply           int32          `json:"reply"`
	Likenum         int16          `json:"likenum"`
	Extra           int32          `json:"extra"`
	Anonymous       int            `json:"anonymous"`
	Protected       int16          `json:"protected"`
	IsTop           models.BoolInt `json:"is_top"`
	Label           int32          `json:"label"`
	Status          int16          `json:"status"`
	IsComment       int            `json:"is_comment"`
	TagsIds         string         `json:"tags_ids"`
	AutoTagsIds     string         `json:"auto_tags_ids"`
	MgrTagsIds      string         `json:"mgr_tags_ids"`
	MediaIds        string         `json:"media_ids"`
	Fold            int16          `json:"fold"`
	Kind            int16          `json:"kind"`
	RewardCost      int16          `json:"reward_cost"`
	RewardState     int16          `json:"reward_state"`
	IdentityShow    int            `json:"identity_show"`
	IdentityType    string         `json:"identity_type"`
	IdentityInfo    interface{}    `json:"identity_info"`
	ExclusiveIdId   int16          `json:"exclusive_id_id"`
	ExclusiveIdInfo interface{}    `json:"exclusive_id_info"`
	Mention         string         `json:"mention"`
	Mailbox         int16          `json:"mailbox"`
	ImageSize       string         `json:"image_size"`
	HasRewardGood   int            `json:"has_reward_good"`
	IsGodHole       int            `json:"is_god_hole"`
	IsProtect       int            `json:"is_protect"`
	TreadNum        int16          `json:"tread_num"`
	PraiseNum       int16          `json:"praise_num"`
	PraiseNumShow   int16          `json:"praise_num_show"`
	FoldNum         int16          `json:"fold_num"`
	IsFold          int            `json:"is_fold"`
	CannotReply     int            `json:"cannot_reply"`
	CommentList     []TempComment  `json:"comment_list"`
}

// TempComment 用于匹配API响应的临时评论结构体
type TempComment struct {
	Anonymous       int         `json:"anonymous"`
	Cid             int32       `json:"cid"`
	Pid             int32       `json:"pid"`
	ExclusiveIdId   int16       `json:"exclusive_id_id"`
	Hidden          int         `json:"hidden"`
	IdentityShow    int         `json:"identity_show"`
	IdentityType    string      `json:"identity_type"`
	IdentityInfo    interface{} `json:"identity_info"`
	IsLz            int         `json:"is_lz"`
	Mention         string      `json:"mention"`
	NameTag         string      `json:"name_tag"`
	RewardGood      int16       `json:"reward_good"`
	Text            string      `json:"text"`
	Timestamp       int32       `json:"timestamp"`
	Quote           interface{} `json:"quote"`
	MediaIds        string      `json:"media_ids"`
	ExclusiveIdInfo interface{} `json:"exclusive_id_info"`
}

func convertTempComments(items []TempComment) []models.Comment {
	comments := make([]models.Comment, 0, len(items))
	for _, temp := range items {
		var quoteID *int32
		if temp.Quote != nil {
			if quoteMap, ok := temp.Quote.(map[string]interface{}); ok {
				if cidVal, ok := quoteMap["cid"].(float64); ok {
					qid := int32(cidVal)
					quoteID = &qid
				}
			}
		}
		comment := models.Comment{
			Anonymous:     models.BoolFromInt(temp.Anonymous),
			Cid:           temp.Cid,
			Pid:           temp.Pid,
			ExclusiveIdId: temp.ExclusiveIdId,
			Hidden:        models.BoolFromInt(temp.Hidden),
			IdentityShow:  models.BoolFromInt(temp.IdentityShow),
			IdentityType:  temp.IdentityType,
			IsLz:          models.BoolFromInt(temp.IsLz),
			Mention:       temp.Mention,
			NameTag:       temp.NameTag,
			RewardGood:    temp.RewardGood,
			Text:          temp.Text,
			Timestamp:     temp.Timestamp,
			QuoteID:       quoteID,
			MediaIds:      temp.MediaIds,
		}
		comment.IdentityInfo = getJSONStringFromInterface(temp.IdentityInfo)
		comment.ExclusiveIdInfo = getJSONStringFromInterface(temp.ExclusiveIdInfo)
		comments = append(comments, comment)
	}
	return comments
}

func fetchPostsV3(c *client.Client, page int, postLimit int, commentLimit int, fetchImages bool, convertWebp bool) ([]models.Post, []models.Comment, map[string]interface{}, error) {
	return fetchPostsV3WithHistory(c, nil, page, postLimit, commentLimit, fetchImages, convertWebp, nil)
}

func fetchPostsV3WithHistory(c *client.Client, database *db.Database, page int, postLimit int, commentLimit int, fetchImages bool, convertWebp bool, previous []models.Post) ([]models.Post, []models.Comment, map[string]interface{}, error) {
	log.Printf("[Crawler] 正在请求 API: page=%d, postLimit=%d, commentLimit=%d", page, postLimit, commentLimit)
	rawResponse, err := fetchPostListRecovering(c, database, page, postLimit, commentLimit, previous)
	if err != nil {
		return nil, nil, nil, err
	}

	data, ok := rawResponse["data"].(map[string]interface{})
	if !ok {
		log.Printf("[Crawler] 无效的 data 格式")
		return nil, nil, nil, fmt.Errorf("invalid data format")
	}

	postsData, ok := data["list"].([]interface{})
	if !ok {
		log.Printf("[Crawler] 无效的 posts data 格式")
		return nil, nil, nil, fmt.Errorf("invalid posts data format")
	}

	// 将帖子数据反序列化到临时结构体
	var tempPosts []TempPost
	postsBytes, err := json.Marshal(postsData)
	if err != nil {
		log.Printf("[Crawler] 序列化帖子数据失败: %v", err)
		return nil, nil, nil, err
	}
	if err := json.Unmarshal(postsBytes, &tempPosts); err != nil {
		log.Printf("[Crawler] 反序列化临时帖子失败: %v", err)
		return nil, nil, nil, err
	}

	var posts []models.Post
	var comments []models.Comment

	// 转换临时结构体到最终模型，保持原始顺序
	for _, tempPost := range tempPosts {
		// 转换帖子
		post := models.Post{
			Pid:           tempPost.Pid,
			Text:          tempPost.Text,
			Type:          tempPost.Type,
			Timestamp:     tempPost.Timestamp,
			Hidden:        models.BoolFromInt(tempPost.Hidden),
			Reply:         tempPost.Reply,
			Likenum:       tempPost.Likenum,
			Extra:         tempPost.Extra,
			Anonymous:     models.BoolFromInt(tempPost.Anonymous),
			Protected:     tempPost.Protected,
			IsTop:         tempPost.IsTop,
			Label:         tempPost.Label,
			Status:        tempPost.Status,
			IsComment:     models.BoolFromInt(tempPost.IsComment),
			TagsIds:       tempPost.TagsIds,
			AutoTagsIds:   tempPost.AutoTagsIds,
			MgrTagsIds:    tempPost.MgrTagsIds,
			MediaIds:      tempPost.MediaIds,
			Fold:          tempPost.Fold,
			Kind:          tempPost.Kind,
			RewardCost:    tempPost.RewardCost,
			RewardState:   tempPost.RewardState,
			IdentityShow:  models.BoolFromInt(tempPost.IdentityShow),
			IdentityType:  tempPost.IdentityType,
			ExclusiveIdId: tempPost.ExclusiveIdId,
			Mention:       tempPost.Mention,
			Mailbox:       tempPost.Mailbox,
			ImageSize:     tempPost.ImageSize,
			HasRewardGood: models.BoolFromInt(tempPost.HasRewardGood),
			IsGodHole:     models.BoolFromInt(tempPost.IsGodHole),
			IsProtect:     models.BoolFromInt(tempPost.IsProtect),
			TreadNum:      tempPost.TreadNum,
			PraiseNum:     tempPost.PraiseNum,
			PraiseNumShow: tempPost.PraiseNumShow,
			FoldNum:       tempPost.FoldNum,
			IsFold:        models.BoolFromInt(tempPost.IsFold),
			CannotReply:   models.BoolFromInt(tempPost.CannotReply),
		}

		// 处理特殊字段
		post.IdentityInfo = getJSONStringFromInterface(tempPost.IdentityInfo)
		post.ExclusiveIdInfo = getJSONStringFromInterface(tempPost.ExclusiveIdInfo)

		// The post list already embeds the first comment_limit comments
		// (ascending by cid). Reply can undercount, so a capped embedded list
		// is paged to the end regardless of reply.
		post.Comments = convertTempComments(tempPost.CommentList)
		capped := commentLimit > 0 && len(post.Comments) >= commentLimit
		if capped || len(post.Comments) < int(tempPost.Reply) {
			target := int(tempPost.Reply)
			if capped {
				target = 0
			}
			post.Comments, err = fetchCommentsFrom(c, post.Pid, post.Comments, target, commentLimit)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("抓取帖子 %d 的评论失败: %w", post.Pid, err)
			}
		}
		comments = append(comments, post.Comments...)

		posts = append(posts, post)
	}

	// Keep the original list response and include the complete fetched comments.
	rawResponse["fetched_comments"] = comments
	if fetchImages {
		for _, post := range posts {
			if post.Type == "image" || post.MediaIds != "" {
				downloadImages(c, post.MediaIds, post.Pid, convertWebp)
			}
			for _, comment := range post.Comments {
				if comment.MediaIds != "" {
					downloadImages(c, comment.MediaIds, 0, convertWebp)
				}
			}
		}
	}

	log.Printf("[Crawler] 数据处理完成: %d 个帖子, %d 个评论", len(posts), len(comments))
	return posts, comments, rawResponse, nil
}

// FetchImagesFromDB 从数据库中查找有图片的帖子和评论，下载缺失的图片
func FetchImagesFromDB(c *client.Client, database *db.Database, convertWebp bool) {
	log.Println("[Crawler] 开始从数据库下载缺失的图片...")

	// 下载帖子图片
	postCount, err := database.GetPostsWithImagesCount()
	if err != nil {
		log.Printf("[Crawler] 查询有图片的帖子数量失败: %v", err)
		return
	}
	log.Printf("[Crawler] 数据库中有 %d 个帖子包含图片", postCount)

	batchSize := 100
	postDownloaded := 0
	postSkipped := 0
	for offset := 0; offset < postCount; offset += batchSize {
		posts, err := database.GetPostsWithImages(offset, batchSize)
		if err != nil {
			log.Printf("[Crawler] 查询帖子失败: %v", err)
			continue
		}

		for _, post := range posts {
			if post.Type == "image" || post.MediaIds != "" {
				d, s := downloadImages(c, post.MediaIds, post.Pid, convertWebp)
				postDownloaded += d
				postSkipped += s
			}
		}
		log.Printf("[Crawler] 帖子图片进度: %d/%d", offset+len(posts), postCount)
	}

	// 下载评论图片
	commentCount, err := database.GetCommentsWithImagesCount()
	if err != nil {
		log.Printf("[Crawler] 查询有图片的评论数量失败: %v", err)
		return
	}
	log.Printf("[Crawler] 数据库中有 %d 个评论包含图片", commentCount)

	commentDownloaded := 0
	commentSkipped := 0
	for offset := 0; offset < commentCount; offset += batchSize {
		comments, err := database.GetCommentsWithImages(offset, batchSize)
		if err != nil {
			log.Printf("[Crawler] 查询评论失败: %v", err)
			continue
		}

		for _, comment := range comments {
			if comment.MediaIds != "" {
				d, s := downloadImages(c, comment.MediaIds, 0, convertWebp)
				commentDownloaded += d
				commentSkipped += s
			}
		}
		log.Printf("[Crawler] 评论图片进度: %d/%d", offset+len(comments), commentCount)
	}

	log.Printf("[Crawler] 图片下载完成! 帖子: 新下载 %d, 跳过 %d | 评论: 新下载 %d, 跳过 %d",
		postDownloaded, postSkipped, commentDownloaded, commentSkipped)
}
func getJSONStringFromInterface(val interface{}) string {
	if val == nil {
		return ""
	}

	// 检查是否为空数组
	if arr, ok := val.([]interface{}); ok && len(arr) == 0 {
		return ""
	}

	if bytes, err := json.Marshal(val); err == nil {
		jsonStr := string(bytes)
		if jsonStr == "[]" {
			return ""
		}
		return jsonStr
	}
	return ""
}

func getStringField(data map[string]interface{}, key string) string {
	if val, exists := data[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// getIntField 安全获取整数字段（处理JSON数字默认为float64的问题）
func getIntField(data map[string]interface{}, key string) int {
	if val, exists := data[key]; exists {
		if floatVal, ok := val.(float64); ok {
			return int(floatVal)
		}
		if intVal, ok := val.(int); ok {
			return intVal
		}
	}
	return 0
}

// downloadImages 下载图片，返回 (新下载数, 跳过数)
func downloadImages(c *client.Client, mediaIDs string, fallbackPID int32, convertWebp bool) (int, int) {
	downloaded := 0
	skipped := 0

	if mediaIDs == "" {
		url := fmt.Sprintf("https://treehole.pku.edu.cn/chapi/api/v3/media/getImageBinary?pid=%d", fallbackPID)
		if saveImage(c, url, fallbackPID, convertWebp) {
			downloaded++
		} else {
			skipped++
		}
		return downloaded, skipped
	}

	ids := strings.Split(mediaIDs, ",")
	for _, idStr := range ids {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		url := fmt.Sprintf("https://treehole.pku.edu.cn/chapi/api/v3/media/getImageBinary?id=%d", id)
		if saveImage(c, url, int32(id), convertWebp) {
			downloaded++
		} else {
			skipped++
		}
	}
	return downloaded, skipped
}

// FetchThumbnailsByIDRange 批量下载缩略图，返回 (新下载数, 跳过数)
func FetchThumbnailsByIDRange(c *client.Client, startID int, endID int, convertWebp bool) (int, int, error) {
	if startID <= 0 || endID <= 0 {
		return 0, 0, fmt.Errorf("media id 必须大于 0")
	}
	if startID > endID {
		return 0, 0, fmt.Errorf("startID(%d) 不能大于 endID(%d)", startID, endID)
	}

	urlTemplate := "https://treehole.pku.edu.cn/chapi/api/v3/media/getThumbnail?id=%d"
	downloaded, skipped := downloadMediaByIDRange(c, startID, endID, urlTemplate, thumbnailDir, convertWebp)
	log.Printf("[Crawler] 缩略图批量下载完成: range=%d-%d, downloaded=%d, skipped=%d", startID, endID, downloaded, skipped)
	return downloaded, skipped, nil
}

func downloadMediaByIDRange(c *client.Client, startID int, endID int, urlTemplate string, outputDir string, convertWebp bool) (int, int) {
	downloaded := 0
	skipped := 0
	for id := startID; id <= endID; id++ {
		url := fmt.Sprintf(urlTemplate, id)
		if saveMedia(c, url, int32(id), outputDir, convertWebp) {
			downloaded++
		} else {
			skipped++
		}
	}
	return downloaded, skipped
}

// saveImage 下载并保存图片文件，返回是否新下载
func saveImage(c *client.Client, url string, mediaID int32, convertWebp bool) bool {
	return saveMedia(c, url, mediaID, imageDir, convertWebp)
}

func saveMedia(c *client.Client, url string, mediaID int32, outputDir string, convertWebp bool) bool {
	ensureDir(outputDir)

	basePath := filepath.Join(outputDir, fmt.Sprintf("%d", mediaID))

	if fileExists(basePath) {
		return false
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("[Crawler] 创建图片请求失败: mediaID=%d, err=%v", mediaID, err)
		return false
	}

	if auth := c.GetAuthorization(); auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}

	resp, err := c.GetHttpClient().Do(req)
	if err != nil {
		log.Printf("[Crawler] 下载图片失败: mediaID=%d, err=%v", mediaID, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[Crawler] 下载图片返回非200: mediaID=%d, status=%d", mediaID, resp.StatusCode)
		return false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[Crawler] 读取图片数据失败: mediaID=%d, err=%v", mediaID, err)
		return false
	}

	contentType := resp.Header.Get("Content-Type")
	isGIF := contentType == "image/gif"

	if isGIF || !convertWebp {
		ext := ".jpg"
		switch contentType {
		case "image/png":
			ext = ".png"
		case "image/gif":
			ext = ".gif"
		case "image/webp":
			ext = ".webp"
		}
		filename := basePath + ext
		if err := os.WriteFile(filename, body, 0644); err != nil {
			log.Printf("[Crawler] 保存图片失败: mediaID=%d, err=%v", mediaID, err)
			return false
		}
		log.Printf("[Crawler] 图片已保存: %s (%d bytes)", filename, len(body))
		return true
	}

	img, format, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		log.Printf("[Crawler] 解码图片失败: mediaID=%d, err=%v", mediaID, err)
		filename := basePath + ".jpg"
		if err := os.WriteFile(filename, body, 0644); err != nil {
			return false
		}
		return true
	}

	filename := basePath + ".webp"
	buf := new(bytes.Buffer)
	if err := webp.Encode(buf, img, &webp.Options{Lossless: false, Quality: 80}); err != nil {
		log.Printf("[Crawler] 编码WebP失败: mediaID=%d, err=%v，回退到原格式", mediaID, err)
		switch format {
		case "png":
			filename = basePath + ".png"
			fw := fileWriter(filename)
			if fw == nil {
				return false
			}
			defer fw.Close()
			if err := png.Encode(fw, img); err != nil {
				return false
			}
		case "jpeg":
			filename = basePath + ".jpg"
			fw := fileWriter(filename)
			if fw == nil {
				return false
			}
			defer fw.Close()
			if err := jpeg.Encode(fw, img, nil); err != nil {
				return false
			}
		default:
			filename = basePath + ".jpg"
			if err := os.WriteFile(filename, body, 0644); err != nil {
				return false
			}
		}
		log.Printf("[Crawler] 图片已保存: %s", filename)
		return true
	}

	if err := os.WriteFile(filename, buf.Bytes(), 0644); err != nil {
		log.Printf("[Crawler] 保存WebP图片失败: mediaID=%d, err=%v", mediaID, err)
		return false
	}

	origSize := len(body)
	newSize := buf.Len()
	saved := 0
	if origSize > 0 {
		saved = (1 - newSize*100/origSize)
	}
	log.Printf("[Crawler] 图片已保存: %s (%d → %d bytes, 节省 %d%%)", filename, origSize, newSize, saved)
	return true
}

func fileWriter(path string) *os.File {
	f, err := os.Create(path)
	if err != nil {
		return nil
	}
	return f
}

// fileExists 检查文件是否存在（尝试所有支持的扩展名）
func fileExists(basePath string) bool {
	exts := []string{".jpg", ".png", ".gif", ".webp"}
	for _, ext := range exts {
		if _, err := os.Stat(basePath + ext); err == nil {
			return true
		}
	}
	return false
}
