# PKUHoleTUI

北京大学树洞命令行数据管理工具

![](doc/assets/dashboard.png)
![](doc/assets/main.png)

## 功能

- **采集**：一次性 / 循环 / 监控抓取，自动记录删帖
- **TUI**：浏览帖子、搜索、看评论，管理后台采集任务
- **搜索**：SQLite FTS5 全文索引
- **API**：查询接口以及treehole v3 只读兼容镜像

## 快速开始

```bash
# TUI + 采集
go build -tags sqlite_fts5 -o treehole ./cmd/

# 加上 API 服务
go build -tags "sqlite_fts5 withserver" -o treehole ./cmd/

./treehole                                      # TUI
./treehole crawler --pages 100 --page-delay 1   # 采集
./treehole server --port 8081                   # API（需 withserver）
```

首次启动会生成 `data/config.json`，填写账号和数据库后即可使用，见[配置](internal/config/README.md)。

## 模块文档

| 模块 | 文档 |
|------|------|
| 命令行 / 后台运行 / 日志 | [cmd/README.md](cmd/README.md) |
| 数据采集 | [internal/crawler/README.md](internal/crawler/README.md) |
| TUI | [internal/tui/README.md](internal/tui/README.md) |
| 数据库与搜索 | [internal/db/README.md](internal/db/README.md) |
| REST API | [server/README.md](server/README.md) |
| 配置 | [internal/config/README.md](internal/config/README.md) |
| 客户端与认证 | [internal/client/README.md](internal/client/README.md) |
| 原站接口字段 | [doc/api.md](doc/api.md) |
| 变更记录 | [CHANGELOG.md](CHANGELOG.md) |
