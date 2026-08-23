# PKUHoleTUI

北京大学树洞命令行数据管理工具

![](doc/assets/dashboard.png)
![](doc/assets/main.png)

## 功能概述

- **数据采集**: 支持一次性抓取、无限循环、断点续爬、持续监控四种模式
- **TUI 交互界面**: 基于 Terminal UI 的可视化操作，支持帖子浏览、搜索、标签筛选、评论查看，以及在线模式下的点赞/关注/发帖/评论
- **全文搜索**: 在 SQLite 中使用 FTS5，支持布尔查询语法（`AND` / `OR` / `-排除`）; PostgreSQL 暂时还没做；
- **REST API 服务**: 提供标准化查询接口，也带了一套与原站 `/chapi/api/v3` 协议兼容的只读镜像路由，相比原站最多加速约 3000x，参见 [benchmark](/doc/benchmark.md)；
- **图片下载**: 支持从帖子和评论中下载缺失图片
- **数据存储**: 支持 SQLite 和 PostgreSQL 双数据库
- **认证管理**: 支持 Cookie / OAuth / SSO / TOTP 多方式登录

## 项目结构

```
.
├── cmd/
│   ├── main.go          # 程序入口
│   ├── root.go          # 根命令 & TUI / Daemon 逻辑
│   ├── crawler.go       # 爬虫子命令 & 图片下载子命令
│   ├── fts.go           # 全文索引
│   ├── server.go        # API 服务器子命令 (build tag: withserver)
│   └── server_stub.go   # server 空壳 (build tag: !withserver)
├── internal/
│   ├── client/          # 树洞 API 客户端（登录、请求、Cookie 管理）
│   ├── config/          # 配置管理（账号、数据库、CORS）
│   ├── crawler/         # 核心爬虫逻辑（TUI 和 Daemon 共用）
│   ├── db/              # 数据库操作（SQLite / PostgreSQL）
│   ├── models/          # 数据模型（Post, Comment）
│   └── tui/             # Terminal UI 界面
│       ├── model.go     # 状态模型
│       ├── update.go    # 消息处理与命令
│       ├── view.go      # 视图渲染
│       ├── styles.go    # 样式定义
│       └── pages/       # 各页面组件
├── server/
│   ├── router.go        # Gin 路由注册（加了 /chapi/api/v3 兼容镜像）
│   ├── handles/         # API 处理器
│   └── utils/           # 工具函数
├── doc/                 # 文档
├── web/                 # 前端静态资源
└── data/
    ├── config.json      # 账号与数据库配置文件（启动时自动创建）
    ├── cookies.json     # 登录凭据（自动生成）
    └── crawler.log      # 运行日志
```

## 安装与构建

### 完整版本（包含 TUI + 数据采集 + API 服务器）

```bash
go mod tidy
go build -tags "sqlite_fts5 withserver" -o treehole ./cmd/
```

### 精简版本（仅 TUI + 数据采集，不包含 API 服务器）

```bash
go mod tidy
go build -tags sqlite_fts5 -o treehole ./cmd/
```

说明：
- **`sqlite_fts5` 是必需的构建标签**：启用 SQLite FTS5 全文搜索，但是数据库会膨胀。不带该 tag 也能运行，但全文搜索会自动降级为 LIKE 扫描；PostgreSQL 后端同样走 LIKE 路径
- 使用 `-tags withserver` 编译时会打包 Gin 框架和所有 API 路由

## 配置

编辑 `data/config.json` 填写登录信息与数据库配置；首次启动若文件不存在，会自动生成默认配置：

```json
{
  "username": "你的学号（可选；无可用 cookie 时用于重新登录）",
  "password": "你的密码（可选；需与 username 配套）",
  "secret_key": "TOTP密钥（可选；遇到令牌验证时自动填写）",
  "database": {
    "type": "sqlite3",
    "db_file": "./data/treehole.db",
    "max_conns": 8
  }
}
```

说明：
- 程序会优先复用 `data/cookies.json` 中的现有登录态。
- 若 cookie 失效，且配置了 `username` + `password`，则会自动尝试 OAuth / SSO 登录。
- TUI 遇到“短信验证”时会弹出验证码输入框；遇到“令牌验证”但未配置 `secret_key` 时会弹出动态口令输入框。
- crawler 为非交互模式；遇到短信验证，或遇到令牌验证但没有可用 `secret_key` 时，会直接退出并提示原因。

支持两种数据库后端：

| 类型 | 配置项 |
|------|--------|
| SQLite | `"type": "sqlite3"`, `"db_file"`, `"max_conns"` |
| PostgreSQL | `"type": "postgres"`, `"host"`, `"port"`, `"user"`, `"password"`, `"name"` |

## 全文搜索

SQLite 后端 + `-tags sqlite_fts5` 构建时，帖子正文会建立 FTS5 倒排索引，搜索走索引而非 LIKE 扫描：

- 中文单字分词，英文正常按单词分词；（为什么不给中文也做分词？存在比较多的网络用语，分词效果不好；而且树洞也不做分词）
- **布尔语法**（TUI 搜索框与 API 的 `keyword` 参数通用）：

| 语法 | 示例 | 含义 |
|------|------|------|
| 隐式 AND | `保研 清华` | 同时包含 |
| `OR` / `\|` | `保研 OR 考研`、`食堂\|开门` | 任一包含 |
| 排除 | `经验 -清华` | 含"经验"且不含"清华" |
| 短语 | `"二食堂 开门"` | 保序相邻匹配 |
| 分组 | `(保研 OR 考研) 资料 -广告` | 括号优先级 |
| 指定帖 | `#8141313 评论` | 限定 pid |

- **自动维护**：爬虫增量数据经触发器实时入索引；存量数据执行一次回填即可；索引 schema 变更会在启动时自动检测并同步重建

索引管理子命令：

```bash
go run -tags sqlite_fts5 ./cmd fts --status    # 查看索引状态与体积
go run -tags sqlite_fts5 ./cmd fts --backfill  # 为存量帖子生成分词列并增量入索引（幂等）
go run -tags sqlite_fts5 ./cmd fts --reindex   # 从分词列全量重建倒排索引
```

## 使用方式

### TUI 模式（默认）

```bash
./treehole

# 把 TUI 的真实终端输出和最新屏幕快照写到目录里
./treehole --tui-capture-dir ./.treehole-tui
```

TUI 主题会默认根据当前终端背景自动选择深色或浅色。也可以手动覆盖：

```bash
TREEHOLE_THEME=dark ./treehole
TREEHOLE_THEME=light ./treehole
TREEHOLE_THEME=auto ./treehole
TREEHOLE_TUI_CAPTURE_DIR=./.treehole-tui ./treehole
```

如果开启了 TUI 捕获，会在指定目录生成：
- `raw-output.ansi`：Bubble Tea 实际写到终端的原始 ANSI 字节流，包含 alt-screen、光标控制和颜色。
- `current-frame.ansi`：最近一次 `View()` 渲染出的完整屏幕内容，保留 ANSI 颜色，适合直接读取当前画面。
- `current-frame.txt`：去掉 ANSI 控制序列后的纯文本快照，便于脚本或调试读取。

界面包含多个页面（`Tab` 切换），初始进入 Dashboard 总览页：
- **同步**: 采集启停控制、模式选择、统计信息
- **帖子**: 帖子列表浏览、搜索、标签筛选、评论查看，以及在线模式下的交互动作
- **课表**: 查看课程安排
- **成绩**: 查看成绩与 GPA

TUI 现在支持两种数据模式：
- **在线模式**：直接调用树洞官方 API，支持读取帖子/评论、标签筛选，以及点赞/关注/发帖/评论
- **离线模式**：回退到本地数据库，只支持浏览；写操作会明确显示为不可用

当在线能力不可用时：
- **登录失败 / 会话失效**：会提示用户选择“重新登录”或“进入离线模式”
- **网络错误 / 服务不可达**：会提示用户进入离线模式

快捷键：

TUI 内置了上下文相关的帮助面板（按 `h` 查看当前页面可用快捷键），底部状态栏也会显示 `h: 帮助 | Ctrl+Q: 退出`。以下列出界面中**未直接展示**的快捷键：

Home 页（爬虫控制，仅在停止状态下可用）：

| 按键 | 功能 |
|------|------|
| `+` `-` | 调整当前模式参数（监控页数 / 每请求帖子数等） |
| `[` `]` | 调整缩略图起始 ID（步长 100） |
| `i` | 切换是否下载图片 |
| `j` | 切换是否保存原始 JSON |
| `w` | 切换是否转换为 WebP 格式 |

Posts 页按 `Esc` 可清除当前搜索词和标签筛选，恢复全部帖子列表（帮助面板中未标注此行为）。

搜索框支持布尔查询（离线模式）：`保研 OR 考研`、`经验 -清华`、`"二食堂 开门"`、`#8141313 关键词`；在线模式走原站搜索接口，语法以原站为准。

配置编辑器支持 Vim 风格操作（`i`/`a`/`o`/`O` 进入编辑模式，`x` 删除字符，`0`/`$`/`g`/`G` 跳转，`Ctrl+S` 保存）。

### 数据采集模式 (`crawler` 子命令)

```bash
# 一次性抓取：从第1页开始，抓100页，每页间隔1秒
./treehole crawler --pages 100 --page-delay 1

# 从第5页开始，抓取50页
./treehole crawler --start-page 5 --pages 50

# 无限循环抓取：从第1页开始一直往后爬
./treehole crawler --page-delay 1

# 断点续采：根据数据库已有帖子数自动计算起始页
./treehole crawler --resume --pages 50

# 循环模式：每轮抓取前10页，每页间隔2秒，每轮间隔60秒
./treehole crawler --pages 10 --loop --page-delay 2 --loop-delay 60

# 下载帖子中的图片
./treehole crawler --pages 50 --fetch-images

# 保存原始 API 响应到 JSON 文件
./treehole crawler --pages 50 --save-raw-json

# 补下载数据库中缺失的图片
./treehole crawler fetch-images
```

采集参数：

| 参数 | 简写 | 说明 | 默认值 |
|------|------|------|--------|
| `--db-path` | | 数据库文件路径 | `./treehole.db` |
| `--start-page` | | 起始页码 | `1` |
| `--pages` | | 每轮抓取页数（0=无限） | `0` |
| `--loop` | | 持续重复抓取轮次 | `false` |
| `--loop-delay` | | 轮与轮之间的间隔（秒） | `60` |
| `--page-delay` | | 页与页之间的间隔（秒） | `1` |
| `--resume` | | 断点续爬 | `false` |
| `--post-batch-size` | | 每次 API 请求最大帖子数 | `200` |
| `--comment-limit` | | 每个帖子最大评论数 | `200` |
| `--fetch-images` | | 下载帖子中的图片 | `false` |
| `--save-raw-json` | | 保存原始 API 响应 | `false` |

### 运行模式对比

| 模式 | 命令 | 适用场景 |
|------|------|---------|
| TUI | `./treehole` | 交互式浏览、搜索、管理 |
| 一次性抓取 | `crawler --pages 100` | 首次全量数据采集 |
| 无限循环 | `crawler` | 持续增量采集 |
| 断点续爬 | `crawler --resume` | 中断后继续 |
| 循环模式 | `crawler --pages 10 --loop` | 同步固定页段（重复轮次） |

### REST API 服务 (`server` 子命令)

```bash
go run -tags "sqlite_fts5 withserver" ./cmd server --port 8081 --host 0.0.0.0
```

自有接口：

- 全部路由: `GET http://localhost:8081/help`
- 帖子列表/搜索: `GET http://localhost:8081/posts?begin=0&limit=25&keyword=&order_by=`
- 帖子详情: `GET http://localhost:8081/post/:pid`
- 单条评论: `GET http://localhost:8081/comment?cid=123`
- 帖子评论: `GET http://localhost:8081/comments/:pid?begin=0&limit=25&sort=0`
- 本地图片: `GET http://localhost:8081/media/image?id=123` 或 `?pid=456`
- 健康检查: `GET http://localhost:8081/health`

原站兼容只读镜像（协议与 `treehole.pku.edu.cn/chapi/api/v3` 一致，可直接对接原站前端）：

- 树洞列表+首页评论: `GET /chapi/api/v3/hole/list_comments?page=1&limit=10&comment_limit=10&keyword=&kind=&label=`
- 单洞+评论: `GET /chapi/api/v3/hole/one?pid=123`
- 单洞信息: `GET /chapi/api/v3/hole/get?pid=123`
- 评论分页: `GET /chapi/api/v3/comment/list?pid=123&page=1&limit=10&sort=0`
- 静态默认值: `/chapi/api/v3/message/un_read`、`/chapi/api/v3/message/index`、`/chapi/api/v3/tags/tree`
- 图片: `/chapi/api/v3/media/getThumbnail`、`/chapi/api/v3/media/getImageBinary`

仅实现只读且与用户状态无关的部分；写操作接口未注册。响应信封为 `{code:20000, data, message:"success", success:true, timestamp}`。延迟对比见 [doc/benchmark.md](doc/benchmark.md)。

### 后台运行

使用 `nohup` 在后台运行采集：

```bash
# 后台监控模式
nohup ./treehole crawler --pages 10 --loop --page-delay 2 --loop-delay 60 &

# 后台无限抓取
nohup ./treehole crawler --page-delay 1 &

# 按 Ctrl+C 或 kill 发送 SIGINT/SIGTERM 可退出
```

## 日志

所有运行日志写入 `crawler.log`，格式为：

```
[2026/04/02 12:00:00.000000] [Daemon] 开始第 1 轮抓取
[2026/04/02 12:00:01.000000] [Daemon] 第 1 页完成: +100帖子 +50评论 | 总计: 1000帖子 500评论
[2026/04/02 12:00:01.000000] [Auth] Cookie 登录成功
```

日志标签：
- `[Crawler]` — 数据采集相关
- `[Auth]` — 登录认证相关
- `[Posts]` — 帖子加载相关
- `[Daemon]` — 后台模式相关

## 技术栈

- **Go 1.26** — 主语言
- **GORM** — ORM 框架（SQLite / PostgreSQL）
- **Bubbletea** — TUI 框架
- **Gin** — Web API 框架
- **Cobra** — CLI 命令框架
- **SQLite / PostgreSQL** — 数据库
