# 数据采集

抓取帖子与评论，记录删帖，支持超时自动恢复。

## 用法

```bash
./treehole crawler --pages 100 --page-delay 1          # 抓 100 页
./treehole crawler --start-page 5 --pages 50           # 从第 5 页开始
./treehole crawler --loop --loop-delay 60              # 持续循环
./treehole crawler --pages 10 --page-delay 2 --loop    # 每轮 10 页
./treehole crawler --pages 50 --fetch-images           # 顺便下载图片
./treehole crawler --pages 50 --save-raw-json          # 保存原始响应
./treehole crawler --deleted-json data/deleted.json    # 输出删帖缺口
```

常用参数：

| 参数 | 说明 | 默认 |
|------|------|------|
| `--start-page` | 起始页 | `1` |
| `--pages` | 每轮页数，0 = 抓到空页为止 | `0` |
| `--loop` / `--loop-delay` | 循环抓取 / 轮间隔（秒） | `false` / `60` |
| `--page-delay` | 页间隔（秒） | `1` |
| `--post-batch-size` | 每页帖子数（1..100） | `100` |
| `--comment-limit` | 帖子列表内嵌评论数（0..100） | `100` |
| `--fetch-images` | 下载图片 | `false` |
| `--save-raw-json` | 保存原始 JSON | `false` |
| `--deleted-json` | 输出删帖缺口文件 | 空 |

## 行为说明

- **删帖记录**：每页评论抓完整后对账，`deleted_at` 记录首次发现删除的时间；远端恢复时自动清除。已删内容保留。
- **页大小上限 100**：帖子与评论单次请求都不能超过 100，超过会报错退出。
- **超时恢复**：网络超时自动重试；服务端 408/504 时自动缩小页大小继续。失败时会保留已抓到的分页到 `recovery_failure_*.json` 方便排查。
