# PKUHoleCrawler

北京大学树洞爬虫与数据管理工具

## 功能概述

- **爬虫采集**: 支持一次性抓取、无限循环、断点续爬、持续监控四种模式
- **TUI 交互界面**: 基于 Terminal UI 的可视化操作，支持帖子浏览、搜索、标签筛选、评论查看，以及在线模式下的点赞/关注/发帖/评论
- **REST API 服务**: 提供标准化的数据查询接口（Gin 框架）
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
│   ├── router.go        # Gin 路由注册
│   ├── handles/         # API 处理器
│   └── utils/           # 工具函数
├── py_beta/             # Python 原型验证（仅供参考）
├── web/                 # 前端静态资源
└── data/
    ├── config.json      # 账号与数据库配置文件（启动时自动创建）
    ├── cookies.json     # 登录凭据（自动生成）
    └── crawler.log      # 运行日志
```

## 安装与构建

### 完整版本（包含 TUI + 爬虫 + API 服务器）

```bash
go mod tidy
go build -tags withserver -o treehole ./cmd/
```

### 精简版本（仅 TUI + 爬虫，不包含 API 服务器）

```bash
go mod tidy
go build -o treehole ./cmd/
```

说明：
- 使用 `-tags withserver` 编译时会打包 Gin 框架和所有 API 路由
- 默认编译（无 tag）会跳过 server 子命令，减少二进制体积和依赖

## 配置

编辑 `data/config.json` 填写登录信息与数据库配置；首次启动若文件不存在，会自动生成默认配置：

```json
{
  "username": "你的学号（可选；无可用 cookie 时用于重新登录）",
  "password": "你的密码（可选；需与 username 配套）",
  "secret_key": "TOTP密钥（可选；遇到令牌验证时自动填写）",
  "database": {
    "type": "sqlite3",
    "db_file": "./treehole.db"
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
| SQLite | `"type": "sqlite3"`, `"db_file": "./treehole.db"` |
| PostgreSQL | `"type": "postgres"`, `"host"`, `"port"`, `"user"`, `"password"`, `"name"` |

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

界面包含多个标签页（Tab 切换）：
- **Home**: 爬虫启停控制、统计信息
- **Posts**: 帖子列表浏览、搜索（`/`）、标签筛选（`t`）、分页、评论查看，以及在线模式下的交互动作
- **Logs**: 运行日志（倒序，最新在前，`r` 刷新）
- **Config**: 编辑账号配置

TUI 现在支持两种数据模式：
- **在线模式**：直接调用树洞官方 API，支持读取帖子/评论、标签筛选，以及点赞/关注/发帖/评论
- **离线模式**：回退到本地数据库，只支持浏览；写操作会明确显示为不可用

当在线能力不可用时：
- **登录失败 / 会话失效**：会提示用户选择“重新登录”或“进入离线模式”
- **网络错误 / 服务不可达**：会提示用户进入离线模式

快捷键：
| 按键 | 功能 |
|------|------|
| `Tab` | 切换标签页 |
| `q` | 退出 |
| `←` `→` | 翻页 / 导航 |
| `↑` `↓` | 列表导航 |
| `Enter` | 确认 / 查看帖子详情 |
| `/` | 搜索帖子 |
| `Esc` | 返回 / 退出搜索 |
| `r` | 刷新日志 |

Posts 页 / 帖子详情页新增快捷键：

| 按键 | 功能 |
|------|------|
| `t` | 打开标签筛选（在线模式） |
| `n` | 发帖（仅可写在线模式） |
| `p` | 点赞 / 取消点赞（帖子详情页，仅可写在线模式） |
| `f` | 关注 / 取消关注（帖子详情页，仅可写在线模式） |
| `c` | 发评论（帖子详情页，仅可写在线模式） |
| `s` | 评论正序 / 逆序切换（帖子详情页） |

说明：
- 帖子列表标题会显示当前是 **[在线]** 还是 **[离线]**
- 帖子详情头部会显示当前帖子的 **赞数/关注数** 以及 **已点赞/未点赞、已关注/未关注**
- 如果当前模式不可写，底部快捷键和状态栏会明确提示不可用

### 爬虫模式 (`crawler` 子命令)

```bash
# 一次性抓取：从第1页开始，抓100页，每页间隔1秒
./treehole crawler --max-pages 100 --page-interval 1

# 从第5页开始，抓取50页
./treehole crawler --start-page 5 --max-pages 50

# 无限循环抓取：从第1页开始一直往后爬
./treehole crawler --page-interval 1

# 断点续爬：根据数据库已有帖子数自动计算起始页
./treehole crawler --resume --max-pages 50

# 监控模式：循环抓取前10页，每页间隔2秒，每轮间隔60秒
./treehole crawler --loop-pages 10 --page-interval 2 --loop-interval 60

# 下载帖子中的图片
./treehole crawler --max-pages 50 --fetch-images

# 保存原始 API 响应到 JSON 文件
./treehole crawler --max-pages 50 --save-json

# 补下载数据库中缺失的图片
./treehole crawler fetch-images
```

爬虫参数：

| 参数 | 简写 | 说明 | 默认值 |
|------|------|------|--------|
| `--db-path` | | 数据库文件路径 | `./treehole.db` |
| `--start-page` | | 起始页码 | `1` |
| `--max-pages` | | 每轮最大页数（0=无限） | `0` |
| `--page-interval` | | 页与页之间的间隔（秒） | `1` |
| `--loop-interval` | | 轮与轮之间的间隔（秒） | `60` |
| `--loop-pages` | `-k` | 循环爬取前 N 页（触发监控模式） | `0` |
| `--resume` | | 断点续爬 | `false` |
| `--posts-per-request` | | 每次 API 请求最大帖子数 | `200` |
| `--comments-per-post` | | 每个帖子最大评论数 | `200` |
| `--fetch-images` | | 下载帖子中的图片 | `false` |
| `--save-json` | | 保存原始 API 响应 | `false` |

### 运行模式对比

| 模式 | 命令 | 适用场景 |
|------|------|---------|
| TUI | `./treehole` | 交互式浏览、搜索、管理 |
| 一次性抓取 | `crawler --max-pages 100` | 首次全量数据采集 |
| 无限循环 | `crawler` | 持续增量采集 |
| 断点续爬 | `crawler --resume` | 中断后继续 |
| 监控模式 | `crawler --loop-pages 10` | 同步最新数据（循环前 N 页） |

### REST API 服务 (`server` 子命令)

```bash
./treehole server --port 8081 --host 0.0.0.0
```

启动后访问：
- 帖子列表: `GET http://localhost:8081/pku_hole?begin=0&limit=25`
- 帖子详情: `GET http://localhost:8081/post/:pid`
- 帖子评论: `GET http://localhost:8081/post/:pid/comments?begin=0&limit=25`
- 搜索: `GET http://localhost:8081/search?q=keyword&begin=0&limit=25`
- 健康检查: `GET http://localhost:8081/health`

### 后台运行

使用 `nohup` 在后台运行爬虫：

```bash
# 后台监控模式
nohup ./treehole crawler --loop-pages 10 --page-interval 2 --loop-interval 60 &

# 后台无限抓取
nohup ./treehole crawler --page-interval 1 &

# 按 Ctrl+C 或 kill 发送 SIGINT/SIGTERM 可优雅退出
```

## 日志

所有运行日志写入 `crawler.log`，格式为：

```
[2026/04/02 12:00:00.000000] [Daemon] 开始第 1 轮抓取
[2026/04/02 12:00:01.000000] [Daemon] 第 1 页完成: +100帖子 +50评论 | 总计: 1000帖子 500评论
[2026/04/02 12:00:01.000000] [Auth] Cookie 登录成功
```

日志标签：
- `[Crawler]` — 爬虫抓取相关
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
