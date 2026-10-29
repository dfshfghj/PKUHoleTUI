# Changelog

## [Unreleased]

### Added
- 按模块拆分文档：`cmd/`、`internal/client|config|crawler|db|models|tui/`、`server/` 各自维护 README，主 README 只保留索引与快速开始。

## [2026-10-30]

### Added
- 评论抓全：不足 `reply` 时继续翻页，直到抓完；
- 删洞标记：`posts.deleted_at` / `comments.deleted_at` 记录首次发现删除的时间；
- 抓取容错：客户端超时重试、HTTP 408/504 减半 `limit` 重算 `page`；
- 恢复失败时将已收集分页、失败响应体与错误写入 `recovery_failure_<时间>.json`；
- `--deleted-json`：分页过程中输出 PID 缺口报告，并与旧报告合并；
- 牛顿法 PID 页码定位；
- 服务端新增的校验：帖子 1-100、评论 0-100，超限直接报错退出。（重要！）

### Changed
- `--pages 0` 语义改为“抓到首个空页”；抓取出错立即结束本轮。
- 原始 JSON 改为 `defer` 保存，所有退出路径都会落盘。
- CLI/TUI 默认页大小改为 100；恢复与定位的 offset 数学统一按服务端有效页大小计算。
- `crawler` 参数校验失败或运行错误时进程以非零码退出。

### Fixed
- 兼容原站 `is_top` 的字符串/空值/null 形式。
- 评论列表响应校验（`code`、`list`、`total`），避免把异常响应当成空评论。
- 修复 CLI 忽略 `Execute` 错误导致退出码恒为 0 的问题。

## [2026-10-07]

### Added
- TUI 同步页重做：爬虫改为后台任务（多任务、独立配置、独立日志、启停/删除/查看日志），参数面板支持内联编辑。
- 鼠标操作：任务行选择、参数切换、滚轮与滚动条。

## [2026-09-30]

### Added
- TUI 鼠标控制。
- 媒体二进制读取路由。

## [2026-09-23]

### Added
- 帖子时间戳范围筛选。

## [2026-08-23]

### Added
- SQLite FTS5 全文索引（中文单字分词、布尔查询）。
- 原站 `/chapi/api/v3` 只读兼容镜像。
- FTS 索引加入 `label` / `kind` 字段。
- 性能基准文档。

## [2026-08-14]

### Added
- MIT License。
