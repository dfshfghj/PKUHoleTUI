# 命令行

默认启动 TUI。

| 命令 | 用途 |
|------|------|
| `treehole` | 启动 TUI |
| `treehole crawler` | 采集数据（见 [crawler 文档](../internal/crawler/README.md)） |
| `treehole server` | 启动 API（需要 `withserver` 构建标签，见 [server 文档](../server/README.md)） |
| `treehole fts` | 查看 / 回填 / 重建全文索引（见 [数据库文档](../internal/db/README.md)） |

```bash
# 构建
go build -tags sqlite_fts5 -o treehole ./cmd/
go build -tags "sqlite_fts5 withserver" -o treehole ./cmd/   # 含 API

# 后台运行
nohup ./treehole crawler --pages 10 --loop --page-delay 2 --loop-delay 60 &
nohup ./treehole crawler --page-delay 1 &

# Ctrl+C 或 SIGTERM 退出
```

日志默认写入 `data/crawler.log`，可用 `TREEHOLE_CRAWLER_LOG` 指定文件。参数错误或运行失败会直接报错退出。
