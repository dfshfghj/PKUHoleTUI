# REST API

只读查询服务，需要 `withserver` 构建标签：

```bash
go build -tags "sqlite_fts5 withserver" -o treehole ./cmd/
./treehole server --port 8081 --host 0.0.0.0
```

## 接口

- `GET /help` — 全部路由
- `GET /posts?begin=0&limit=25&keyword=` — 帖子列表/搜索
- `GET /post/:pid` — 帖子详情
- `GET /comments/:pid?begin=0&limit=25&sort=0` — 评论
- `GET /comment?cid=123` — 单条评论
- `GET /media/image?id=123` — 本地图片
- `GET /health` — 健康检查

## 原站 v3 只读镜像

路径与 `treehole.pku.edu.cn/chapi/api/v3` 一致，可直接对接原站前端：

`/hole/list_comments`、`/hole/one`、`/hole/get`、`/comment/list`、`/message/un_read`、`/message/index`、`/tags/tree`、`/media/getThumbnail`、`/media/getImageBinary`。

写接口不实现。字段说明见 [doc/api.md](../doc/api.md)。
