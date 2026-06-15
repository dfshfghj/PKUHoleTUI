package models

import (
	"database/sql/driver"
	"strconv"
)

// BoolInt 数据库存储为 int(0/1)，Go 中使用 bool，JSON 序列化保持 0/1 兼容
type BoolInt bool

func (b BoolInt) Value() (driver.Value, error) {
	return bool(b), nil
}

func (b *BoolInt) Scan(value interface{}) error {
	switch v := value.(type) {
	case int64:
		*b = v != 0
	case int:
		*b = v != 0
	case bool:
		*b = BoolInt(v)
	default:
		*b = false
	}
	return nil
}

func (b BoolInt) MarshalJSON() ([]byte, error) {
	if b {
		return []byte("1"), nil
	}
	return []byte("0"), nil
}

func (b *BoolInt) UnmarshalJSON(data []byte) error {
	s, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	*b = s != 0
	return nil
}

func BoolFromInt(i int) BoolInt {
	return BoolInt(i != 0)
}

type Post struct {
	Pid             int32     `json:"pid" gorm:"primaryKey;column:pid"`
	Text            string    `json:"text" gorm:"column:text"`
	Type            string    `json:"type" gorm:"column:type"`
	Timestamp       int32     `json:"timestamp" gorm:"column:timestamp"`
	Hidden          BoolInt   `json:"hidden" gorm:"column:hidden"`
	Reply           int16     `json:"reply" gorm:"column:reply;index:idx_posts_reply"`
	Likenum         int16     `json:"likenum" gorm:"column:likenum;index:idx_posts_likenum"`
	Extra           int32     `json:"extra" gorm:"column:extra"`
	Anonymous       BoolInt   `json:"anonymous" gorm:"column:anonymous"`
	Protected       int16     `json:"protected" gorm:"column:protected"`
	IsTop           BoolInt   `json:"is_top" gorm:"column:is_top"`
	Label           int16     `json:"label" gorm:"column:label"`
	Status          int16     `json:"status" gorm:"column:status"`
	IsComment       BoolInt   `json:"is_comment" gorm:"column:is_comment"`
	TagsIds         string    `json:"tags_ids" gorm:"column:tags_ids"`
	AutoTagsIds     string    `json:"auto_tags_ids" gorm:"column:auto_tags_ids"`
	MgrTagsIds      string    `json:"mgr_tags_ids" gorm:"column:mgr_tags_ids"`
	MediaIds        string    `json:"media_ids" gorm:"column:media_ids"`
	Fold            int16     `json:"fold" gorm:"column:fold"`
	Kind            int16     `json:"kind" gorm:"column:kind"`
	RewardCost      int16     `json:"reward_cost" gorm:"column:reward_cost"`
	RewardState     int16     `json:"reward_state" gorm:"column:reward_state"`
	IdentityShow    BoolInt   `json:"identity_show" gorm:"column:identity_show"`
	IdentityType    string    `json:"identity_type" gorm:"column:identity_type"`
	IdentityInfo    string    `json:"-" gorm:"column:identity_info"` // 存储为JSON字符串
	ExclusiveIdId   int16     `json:"exclusive_id_id" gorm:"column:exclusive_id_id"`
	ExclusiveIdInfo string    `json:"-" gorm:"column:exclusive_id_info"` // 存储为JSON字符串
	Mention         string    `json:"mention" gorm:"column:mention"`
	Mailbox         int16     `json:"mailbox" gorm:"column:mailbox"`
	ImageSize       string    `json:"image_size" gorm:"column:image_size"`
	HasRewardGood   BoolInt   `json:"has_reward_good" gorm:"column:has_reward_good"`
	IsGodHole       BoolInt   `json:"is_god_hole" gorm:"column:is_god_hole"`
	IsProtect       BoolInt   `json:"is_protect" gorm:"column:is_protect"`
	TreadNum        int16     `json:"tread_num" gorm:"column:tread_num"`
	PraiseNum       int16     `json:"praise_num" gorm:"column:praise_num;index:idx_posts_praise_num"`
	PraiseNumShow   int16     `json:"praise_num_show" gorm:"column:praise_num_show"`
	FoldNum         int16     `json:"fold_num" gorm:"column:fold_num"`
	IsFold          BoolInt   `json:"is_fold" gorm:"column:is_fold"`
	CannotReply     BoolInt   `json:"cannot_reply" gorm:"column:cannot_reply"`
	IsFollow        BoolInt   `json:"is_follow,omitempty" gorm:"-"`
	IsPraise        BoolInt   `json:"is_praise,omitempty" gorm:"-"`
	Comments        []Comment `json:"comment_list,omitempty" gorm:"foreignKey:Pid"`
}

type ExclusiveIdInfo struct {
	Id              int    `json:"id" gorm:"primaryKey;column:id"`
	ExclusiveId     string `json:"exclusive_id" gorm:"column:exclusive_id"`
	ApplicationTime string `json:"application_time" gorm:"column:application_time"`
	IsV             int16  `json:"is_v" gorm:"column:is_v"`
	AuditStatus     string `json:"audit_status" gorm:"column:audit_status"`
	AuditTime       string `json:"audit_time" gorm:"column:audit_time"`
	IsDel           int16  `json:"is_del" gorm:"column:is_del"`
	CreatedAt       string `json:"created_at" gorm:"column:created_at"`
	UpdatedAt       string `json:"updated_at" gorm:"column:updated_at"`
}

type Comment struct {
	Anonymous       BoolInt  `json:"anonymous" gorm:"column:anonymous"`
	Cid             int32    `json:"cid" gorm:"primaryKey;column:cid"`
	Pid             int32    `json:"pid" gorm:"column:pid"`
	ExclusiveIdId   int16    `json:"exclusive_id_id" gorm:"column:exclusive_id_id"`
	Hidden          BoolInt  `json:"hidden" gorm:"column:hidden"`
	IdentityShow    BoolInt  `json:"identity_show" gorm:"column:identity_show"`
	IdentityType    string   `json:"identity_type" gorm:"column:identity_type"`
	IdentityInfo    string   `json:"-" gorm:"column:identity_info"` // 存储为JSON字符串
	IsLz            BoolInt  `json:"is_lz" gorm:"column:is_lz"`
	Mention         string   `json:"mention" gorm:"column:mention"`
	NameTag         string   `json:"name_tag" gorm:"column:name_tag"`
	RewardGood      int16    `json:"reward_good" gorm:"column:reward_good"`
	Text            string   `json:"text" gorm:"column:text"`
	Timestamp       int32    `json:"timestamp" gorm:"column:timestamp"`
	QuoteID         *int32   `json:"-" gorm:"column:quote_id;constraint:OnDelete:SET NULL;"`
	Quote           *Comment `json:"quote,omitempty" gorm:"foreignKey:QuoteID"`
	MediaIds        string   `json:"media_ids" gorm:"column:media_ids"`
	ExclusiveIdInfo string   `json:"-" gorm:"column:exclusive_id_info"` // 存储为JSON字符串
	IsAuthor        BoolInt  `json:"is_author,omitempty" gorm:"-"`
}

type Tag struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Label    string `json:"label"`
	ParentID int    `json:"parent_id"`
}
