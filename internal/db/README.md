# 数据库与搜索

支持 SQLite 与 PostgreSQL，数据表由程序自动创建/迁移。

- `posts` / `comments`：帖子与评论；已删除的内容会保留。
- `deleted_at`：0 表示存活，非 0 为发现删除的时间。
- 删除与恢复由爬虫在抓取完整评论后自动处理。

## 全文搜索

使用 `-tags sqlite_fts5` 构建时启用 FTS5；否则自动降级为 LIKE 扫描（PostgreSQL 同样走 LIKE）。

支持布尔语法（TUI 与 API 通用）：

| 语法 | 示例 | 含义 |
|------|------|------|
| 隐式 AND | `保研 清华` | 同时包含 |
| OR / `\|` | `保研 OR 考研` | 任一包含 |
| 排除 | `经验 -清华` | 含“经验”不含“清华” |
| 短语 | `"二食堂 开门"` | 按顺序相邻 |
| 分组 | `(保研 OR 考研) 资料` | 括号优先 |
| 指定帖 | `#8141313 评论` | 限定 pid |

索引管理：

```bash
go run -tags sqlite_fts5 ./cmd fts --status    # 状态
go run -tags sqlite_fts5 ./cmd fts --backfill  # 存量回填（可重复执行）
go run -tags sqlite_fts5 ./cmd fts --reindex   # 全量重建
```
