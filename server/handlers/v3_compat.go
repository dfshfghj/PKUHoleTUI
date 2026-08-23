package handles

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"treehole/internal/db"
	"treehole/internal/models"

	"github.com/gin-gonic/gin"
)

func v3OK(c *gin.Context, data interface{}) {
	c.JSON(200, gin.H{
		"code":      20000,
		"data":      data,
		"message":   "success",
		"success":   true,
		"timestamp": time.Now().Unix(),
	})
}

func v3Fail(c *gin.Context, code int, message string) {
	c.JSON(200, gin.H{
		"code":      code,
		"data":      nil,
		"message":   message,
		"success":   false,
		"timestamp": time.Now().Unix(),
	})
}

func jsonOrEmptyArray(s string) json.RawMessage {
	s = strings.TrimSpace(s)
	if s == "" {
		return []byte("[]")
	}
	if json.Valid([]byte(s)) {
		return json.RawMessage(s)
	}
	return []byte("[]")
}

func v3Hole(p *models.Post) gin.H {
	var identityInfo interface{} = jsonOrEmptyArray(p.IdentityInfo)
	var exclusiveIDInfo interface{} = jsonOrEmptyArray(p.ExclusiveIdInfo)
	return gin.H{
		"pid":               p.Pid,
		"text":              p.Text,
		"type":              p.Type,
		"timestamp":         p.Timestamp,
		"hidden":            p.Hidden,
		"reply":             p.Reply,
		"likenum":           p.Likenum,
		"extra":             p.Extra,
		"anonymous":         p.Anonymous,
		"hot":               p.Timestamp,
		"tag":               "",
		"protected":         p.Protected,
		"is_top":            p.IsTop,
		"label":             p.Label,
		"status":            p.Status,
		"is_comment":        p.IsComment,
		"tags_ids":          p.TagsIds,
		"auto_tags_ids":     p.AutoTagsIds,
		"mgr_tags_ids":      p.MgrTagsIds,
		"media_ids":         p.MediaIds,
		"fold":              p.Fold,
		"kind":              p.Kind,
		"reward_cost":       p.RewardCost,
		"reward_state":      p.RewardState,
		"identity_show":     p.IdentityShow,
		"identity_type":     p.IdentityType,
		"exclusive_id_id":   int(p.ExclusiveIdId),
		"mention":           p.Mention,
		"mailbox":           p.Mailbox,
		"image_size":        p.ImageSize,
		"has_reward_good":   p.HasRewardGood,
		"tags_info":         []interface{}{},
		"tags_list":         []interface{}{},
		"exclusive_id_info": exclusiveIDInfo,
		"identity_info":     identityInfo,
		"is_god_hole":       p.IsGodHole,
		"is_protect":        p.IsProtect,
		"tread_num":         p.TreadNum,
		"praise_num":        p.PraiseNum,
		"praise_num_show":   p.PraiseNumShow,
		"fold_num":          p.FoldNum,
		"is_sdss":           0,
		"is_follow":         0,
		"attention_info":    []interface{}{},
		"is_praise":         0,
		"is_tread":          0,
		"islz":              0,
		"user_fold":         0,
		"user_config_fold":  1,
		"is_fold":           p.IsFold,
		"cannot_reply":      p.CannotReply,
	}
}

func v3Comment(cmt *models.Comment, depth int) gin.H {
	nameTag := cmt.NameTag
	if nameTag == "" {
		nameTag = "Alice"
	}
	var quote interface{} = []interface{}{}
	if cmt.Quote != nil && depth < 2 {
		quote = v3Comment(cmt.Quote, depth+1)
	}
	var exclusiveIDInfo interface{} = jsonOrEmptyArray(cmt.ExclusiveIdInfo)
	var identityInfo interface{} = jsonOrEmptyArray(cmt.IdentityInfo)
	return gin.H{
		"cid":               cmt.Cid,
		"pid":               cmt.Pid,
		"text":              cmt.Text,
		"timestamp":         cmt.Timestamp,
		"hidden":            cmt.Hidden,
		"anonymous":         cmt.Anonymous,
		"tag":               nil,
		"comment_id":        cmt.QuoteID,
		"name_tag":          nameTag,
		"media_ids":         cmt.MediaIds,
		"reward_good":       cmt.RewardGood,
		"identity_show":     cmt.IdentityShow,
		"identity_type":     cmt.IdentityType,
		"exclusive_id_id":   int(cmt.ExclusiveIdId),
		"mention":           cmt.Mention,
		"quote":             quote,
		"exclusive_id_info": exclusiveIDInfo,
		"identity_info":     identityInfo,
		"is_author":         0,
		"is_lz":             cmt.IsLz,
	}
}

func v3Comments(comments []models.Comment) []gin.H {
	out := make([]gin.H, len(comments))
	for i := range comments {
		out[i] = v3Comment(&comments[i], 0)
	}
	return out
}

func v3QueryInt(c *gin.Context, key string, def, max int) int {
	v, err := strconv.Atoi(c.Query(key))
	if err != nil || v < 1 {
		return def
	}
	if v > max {
		return max
	}
	return v
}

func V3ListComments(database *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		page := v3QueryInt(c, "page", 1, 100000)
		limit := v3QueryInt(c, "limit", 10, 100)
		commentLimit := v3QueryInt(c, "comment_limit", 10, 50)

		keyword := c.Query("keyword")
		var kind, label *int
		if s := c.Query("kind"); s != "" {
			if v, err := strconv.Atoi(s); err == nil {
				kind = &v
			}
		}
		if s := c.Query("label"); s != "" {
			if v, err := strconv.Atoi(s); err == nil {
				label = &v
			}
		}

		posts, total, err := database.GetPostsV3Page(page, limit, keyword, kind, label)
		if err != nil {
			v3Fail(c, 50000, "查询失败: "+err.Error())
			return
		}

		pids := make([]int32, len(posts))
		for i := range posts {
			pids[i] = posts[i].Pid
		}
		commentMap, err := database.GetCommentsForPids(pids, commentLimit)
		if err != nil {
			v3Fail(c, 50000, "查询失败: "+err.Error())
			return
		}

		list := make([]gin.H, len(posts))
		for i := range posts {
			hole := v3Hole(&posts[i])
			hole["comment_list"] = v3Comments(commentMap[posts[i].Pid])
			hole["comment_total"] = posts[i].Reply
			list[i] = hole
		}
		v3OK(c, gin.H{"list": list, "total": total})
	}
}

func V3HoleOne(database *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		pid, err := strconv.ParseInt(c.Query("pid"), 10, 32)
		if err != nil {
			v3Fail(c, 40000, "无效的 pid")
			return
		}
		post, err := database.GetPostByPid(int32(pid))
		if err != nil {
			v3Fail(c, 40400, "树洞不存在")
			return
		}
		commentLimit := v3QueryInt(c, "comment_limit", 10, 50)
		comments, _ := database.GetCommentsByPidCursor(post.Pid, 0, commentLimit, true)
		total, _ := database.CountCommentsByPid(post.Pid)
		v3OK(c, gin.H{
			"hole":  v3Hole(post),
			"list":  v3Comments(comments),
			"total": total,
		})
	}
}

func V3HoleGet(database *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		pid, err := strconv.ParseInt(c.Query("pid"), 10, 32)
		if err != nil {
			v3Fail(c, 40000, "无效的 pid")
			return
		}
		post, err := database.GetPostByPid(int32(pid))
		if err != nil {
			v3Fail(c, 40400, "树洞不存在")
			return
		}
		v3OK(c, v3Hole(post))
	}
}

func V3CommentList(database *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		pid, err := strconv.ParseInt(c.Query("pid"), 10, 32)
		if err != nil {
			v3Fail(c, 40000, "无效的 pid")
			return
		}
		page := v3QueryInt(c, "page", 1, 100000)
		limit := v3QueryInt(c, "limit", 10, 100)
		sortAsc := c.DefaultQuery("sort", "0") != "1"

		comments, err := database.GetCommentsByPidPage(int32(pid), page, limit, sortAsc)
		if err != nil {
			v3Fail(c, 50000, "查询失败: "+err.Error())
			return
		}
		total, _ := database.CountCommentsByPid(int32(pid))
		v3OK(c, gin.H{"list": v3Comments(comments), "total": total})
	}
}

func V3MessageUnread(c *gin.Context) {
	v3OK(c, gin.H{"count": 0})
}

func V3MessageIndex(c *gin.Context) {
	v3OK(c, gin.H{"list": []interface{}{}, "total": 0})
}

func V3TagsTree(c *gin.Context) {
	v3OK(c, []interface{}{})
}
