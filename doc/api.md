# API

## check-health同时查看未读message
https://treehole.pku.edu.cn/chapi/api/v3/message/un_read?message_type=int_msg
https://treehole.pku.edu.cn/chapi/api/v3/message/un_read?message_type=sys_msg

示例：unread_messages.json

## 获取消息
https://treehole.pku.edu.cn/chapi/api/v3/message/index?page=1&limit=10&message_type=int_msg
https://treehole.pku.edu.cn/chapi/api/v3/message/index?page=1&limit=10&message_type=sys_msg

示例：int_messages.json / sys_messages.json

## 获取树洞列表及评论
https://treehole.pku.edu.cn/chapi/api/v3/hole/list_comments?pid=7999240&page=1&limit=10&comment_limit=10&keyword=zkc&label=1&is_follow=1&kind=1&comment_stream=1

kind表示树洞类型，0为普通树洞，1为悬赏树洞，is_follow表示是否关注树洞, label表示标签的编号，comment_stream功能暂不明确，固定带着

示例：list_comments.json

## 获取单个帖子
https://treehole.pku.edu.cn/chapi/api/v3/hole/one?pid=8110393&comment_stream=1

示例：one.json

## 获取评论
https://treehole.pku.edu.cn/chapi/api/v3/comment/list?pid=8127902&page=1&limit=10&sort=0&comment_stream=1

示例：comment.json

## 关注/取消关注（同一接口）
POST https://treehole.pku.edu.cn/chapi/api/v3/hole/attention
eg: {"pid":8141313}

示例：attention_0.json / attention_1.json

## 点赞/取消赞（同一接口）
POST https://treehole.pku.edu.cn/chapi/api/v3/hole/praise
eg: {"pid":8141313}

示例：praise.json （返回数据相同）

## 获取帖子信息（web中在点赞/取消赞/关注/取消关注之后会调用）
https://treehole.pku.edu.cn/chapi/api/v3/hole/get?pid=8141313

示例：get.json

## 获取图片缩略图(media_id/pid)
https://treehole.pku.edu.cn/chapi/api/v3/media/getThumbnail?id=16504
https://treehole.pku.edu.cn/chapi/api/v3/media/getThumbnail?pid=8141302

## 获取图片原图(media_id/pid)
https://treehole.pku.edu.cn/chapi/api/v3/media/getImageBinary?id=16504
https://treehole.pku.edu.cn/chapi/api/v3/media/getImageBinary?pid=8141306

## 获取tag列表
https://treehole.pku.edu.cn/chapi/api/v3/tags/tree

示例：tags.json

## 发布前保存草稿
POST https://treehole.pku.edu.cn/chapi/api/v3/hole_draft/add
eg: {
    "type": "text",
    "kind": 0,
    "reward_cost": 1,
    "text": "test",
    "identity_show": 0,
    "identity_type": "",
    "exclusive_id_id": "",
    "fold": 0,
    "mailbox": 0,
    "tags_ids": "",
    "media_ids": ""
}

## 发布树洞
POST https://treehole.pku.edu.cn/chapi/api/v3/hole/post
eg: {
    "type": "text",
    "kind": 0,
    "reward_cost": 1,
    "text": "test",
    "identity_show": 0,
    "identity_type": "",
    "exclusive_id_id": "",
    "fold": 0,
    "mailbox": 0,
    "tags_ids": "",
    "media_ids": ""
}

## 发布评论
POST https://treehole.pku.edu.cn/chapi/api/v3/comment/post
eg: {
    "pid": 8139378,
    "comment_id": "",
    "text": "test",
    "media_ids": "",
    "identity_show": 0,
    "identity_type": ""
}

# 数据结构解读

## post
eg: {
    "pid": 8139378,
    "text": "zhf思修\n哪来的嘉豪啊😓\n一直追着台上pre的同学要个立场是何意味，给了台阶都不下说是。\n你都表明自己观点了那追着人家个只能打圆场的问有什么意义吗？那我只能归结于你想要炫耀自己的与众不同的观点了",
    "type": "text",
    "timestamp": 1775733384,
    "hidden": 0,
    "reply": 104,
    "likenum": 67,
    "extra": 0,
    "anonymous": 1,
    "hot": 1775733384,
    "tag": null,
    "protected": 0,
    "is_top": 0,
    "label": 191,
    "status": 0,
    "is_comment": 1,
    "tags_ids": "191",
    "auto_tags_ids": "191",
    "mgr_tags_ids": "",
    "media_ids": "",
    "fold": 0,
    "kind": 0,
    "reward_cost": 0,
    "reward_state": 0,
    "identity_show": 0,
    "identity_type": "",
    "exclusive_id_id": 0,
    "mention": "",
    "mailbox": 0,
    "image_size": "",
    "has_reward_good": 0,
    "tags_info": [],
    "tags_list": [],
    "exclusive_id_info": [],
    "identity_info": [],
    "is_god_hole": 1,
    "is_protect": 0,
    "tread_num": 0,
    "praise_num": 0,
    "praise_num_show": 0,
    "fold_num": 0,
    "is_sdss": 0,
    "is_follow": 0,
    "attention_info": [],
    "is_praise": 0,
    "is_tread": 0,
    "islz": 0,
    "user_fold": 0,
    "user_config_fold": 1,
    "is_fold": 0,
    "cannot_reply": 0
}


要点：
+ "hot" 与 "timestamp" 相同
+ "label" 其实是tag的语义，"tag" 字段似乎无用
+ "is_god_hole"表示热度高，判断标准尚不明确
+ "auto_tags_ids"是后台自动打的tag
+ "tags_info"，"tags_list"，"exclusive_id_info"，"identity_info"，"attention_info" 几个字段空时为[]，非空时为{xxx:xxx}的对象，解析时需要注意
+ "likenum"实为关注数，"is_follow"是用户是否已关注，"praise_num"才是点赞，"praise_num_show"总是与"praise_num"相同
+ "kind"是树洞类型，0为普通树洞，1为悬赏树洞，与reward相关的字段联系

## comment
eg: {
    "cid": 37458733,
    "pid": 8139378,
    "text": "666还在群里追着杀😓",
    "timestamp": 1775733723,
    "hidden": 0,
    "anonymous": 1,
    "tag": null,
    "comment_id": null,
    "name_tag": "洞主",
    "media_ids": "16204",
    "reward_good": 0,
    "identity_show": 0,
    "identity_type": "",
    "exclusive_id_id": 0,
    "mention": "",
    "quote": [],
    "exclusive_id_info": [],
    "identity_info": [],
    "is_author": 0,
    "is_lz": 1
}

要点：
+ "is_lz"是否楼主
+ "is_author"表示当前用户是否是该评论的作者
+ "quote"，"exclusive_id_info"，"identity_info"，字段为空时为[]，非空时为{xxx:xxx}的对象，解析时需要注意

总之里面有很多旧版本的遗产，语义不一致的遗留问题，务必注意

更多的示例放在doc/comments_examples.json,doc/post_examples.json中

# 认证
把cookies都带上，headers中有uuid,x-xsrf-token,authorization