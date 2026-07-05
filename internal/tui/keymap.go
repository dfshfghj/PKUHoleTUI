package tui

import (
	"strings"

	"treehole/internal/models"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

type KeyMode string

const (
	KeyModeDashboard       KeyMode = "normal:dashboard"
	KeyModeHome            KeyMode = "normal:home"
	KeyModePostsList       KeyMode = "normal:posts:list"
	KeyModePostsSearch     KeyMode = "search:posts"
	KeyModePostsDetail     KeyMode = "normal:posts:detail"
	KeyModeSchedule        KeyMode = "normal:schedule"
	KeyModeScores          KeyMode = "normal:scores"
	KeyModeHelp            KeyMode = "normal:help"
	KeyModeTools           KeyMode = "normal:tools"
	KeyModeToolsConfig     KeyMode = "normal:tools:config"
	KeyModeToolsConfigEdit KeyMode = "insert:tools:config"
	KeyModeImage           KeyMode = "normal:image"
	KeyModeSession         KeyMode = "normal:session"
	KeyModeSessionInsert   KeyMode = "insert:session"
	KeyModeAuthInsert      KeyMode = "insert:auth"
	KeyModeComposerInsert  KeyMode = "insert:composer"
	KeyModeTags            KeyMode = "normal:tags"
)

type Leader struct {
	Help          key.Binding
	Config        key.Binding
	Logs          key.Binding
	Notifications key.Binding
	Quit          key.Binding
	Praise        key.Binding
	Follow        key.Binding
	Image         key.Binding
	Tags          key.Binding
	Post          key.Binding
	Comment       key.Binding
	Home          key.Binding
	PageHome      key.Binding
	PagePosts     key.Binding
	PageSchedule  key.Binding
	PageScores    key.Binding
}

type Direct struct {
	Close                  key.Binding
	CloseHelp              key.Binding
	DashboardHelp          key.Binding
	ToolsHelp              key.Binding
	Quit                   key.Binding
	Tab                    key.Binding
	ShiftTab               key.Binding
	Refresh                key.Binding
	OpenDetail             key.Binding
	Search                 key.Binding
	Tags                   key.Binding
	Logs                   key.Binding
	Notifications          key.Binding
	DashboardNotifications key.Binding
	DashboardConfig        key.Binding
	Explore                key.Binding
	Post                   key.Binding
	Image                  key.Binding
	Praise                 key.Binding
	Follow                 key.Binding
	Comment                key.Binding
	Reply                  key.Binding
	Sort                   key.Binding
	Move                   key.Binding
	Page                   key.Binding
	LeftRight              key.Binding
	StartStop              key.Binding
	ModeNumbers            key.Binding
	ModeCycle              key.Binding
	Backspace              key.Binding
	GoToTop                key.Binding
}

type ShortCut struct {
	Escape key.Binding
	Enter  key.Binding
	Save   key.Binding
	Resend key.Binding

	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	PgUp     key.Binding
	PgDown   key.Binding
	Tab      key.Binding
	ShiftTab key.Binding

	VimUp    key.Binding
	VimDown  key.Binding
	VimLeft  key.Binding
	VimRight key.Binding

	Increase key.Binding
	Decrease key.Binding

	ToolConfig      key.Binding
	ToolLogs        key.Binding
	ToolInteractive key.Binding
	ToolSystem      key.Binding
	ToolHelp        key.Binding

	MarkAllRead key.Binding
	Clear       key.Binding

	ConfigInsert    key.Binding
	ConfigAppend    key.Binding
	ConfigOpenBelow key.Binding
	ConfigOpenAbove key.Binding
	ConfigDelete    key.Binding
	ConfigLineStart key.Binding
	ConfigLineEnd   key.Binding
	ConfigGo        key.Binding
	ConfigDocBottom key.Binding

	HomeModeCycle     key.Binding
	HomeFetchImages   key.Binding
	HomeSaveJSON      key.Binding
	HomeConvertWebp   key.Binding
	HomeThumbnailPrev key.Binding
	HomeThumbnailNext key.Binding
}

func newKeyBinding(keys []string, helpKey, desc string) key.Binding {
	return key.NewBinding(
		key.WithKeys(keys...),
		key.WithHelp(helpKey, desc),
	)
}

var keymap = struct {
	Prefix struct {
		Leader  key.Binding
		Command key.Binding
	}
	CommandInput struct {
		Cancel key.Binding
		Submit key.Binding
		Delete key.Binding
	}
	Leader Leader
	Direct Direct
}{
	Prefix: struct {
		Leader  key.Binding
		Command key.Binding
	}{
		Leader:  newKeyBinding([]string{" ", "space"}, "Space", "leader"),
		Command: newKeyBinding([]string{":"}, ":", "命令"),
	},
	CommandInput: struct {
		Cancel key.Binding
		Submit key.Binding
		Delete key.Binding
	}{
		Cancel: newKeyBinding([]string{"esc"}, "Esc", "取消命令"),
		Submit: newKeyBinding([]string{"enter"}, "Enter", "执行命令"),
		Delete: newKeyBinding([]string{"backspace"}, "Backspace", "删除字符"),
	},
	Leader: Leader{
		Help:          newKeyBinding([]string{"?"}, "Space ?", "项目帮助"),
		Config:        newKeyBinding([]string{"c"}, "Space c", "打开配置"),
		Logs:          newKeyBinding([]string{"l"}, "Space l", "查看日志"),
		Notifications: newKeyBinding([]string{"n"}, "Space n", "查看通知"),
		Quit:          newKeyBinding([]string{"q"}, "Space q", "退出/返回"),
		// Praise:        newKeyBinding([]string{"p"}, "Space p", "点赞"),
		// Follow:        newKeyBinding([]string{"f"}, "Space f", "关注"),
		// Image:         newKeyBinding([]string{"o"}, "Space o", "打开图片"),
		// Tags:          newKeyBinding([]string{"t"}, "Space t", "标签筛选"),
		// Post:          newKeyBinding([]string{"w"}, "Space w", "发布帖子"),
		// Comment:       newKeyBinding([]string{"r"}, "Space r", "发表评论"),
		// Home:          newKeyBinding([]string{"h"}, "Space h", "回到同步页"),
		// PageHome:      newKeyBinding([]string{"1"}, "Space 1", "同步页"),
		// PagePosts:     newKeyBinding([]string{"2"}, "Space 2", "帖子页"),
		// PageSchedule:  newKeyBinding([]string{"3"}, "Space 3", "课表页"),
		// PageScores:    newKeyBinding([]string{"4"}, "Space 4", "成绩页"),
	},
	Direct: Direct{
		Close:                  newKeyBinding([]string{"esc"}, "Esc", "关闭"),
		CloseHelp:              newKeyBinding([]string{"esc"}, "Esc", "关闭帮助"),
		DashboardHelp:          newKeyBinding([]string{"?"}, "?", "项目帮助"),
		ToolsHelp:              newKeyBinding([]string{"?"}, "?", "当前页面快捷键"),
		Quit:                   newKeyBinding([]string{"q"}, "q", "退出"),
		Tab:                    newKeyBinding([]string{"tab"}, "Tab", "下一页"),
		ShiftTab:               newKeyBinding([]string{"shift+tab"}, "Shift+Tab", "上一页"),
		Refresh:                newKeyBinding([]string{"r"}, "r", "刷新"),
		OpenDetail:             newKeyBinding([]string{"enter"}, "Enter", "打开详情"),
		Search:                 newKeyBinding([]string{"/"}, "/", "搜索帖子"),
		Tags:                   newKeyBinding([]string{"t"}, "t", "标签筛选"),
		DashboardNotifications: newKeyBinding([]string{"n"}, "n", "打开通知"),
		DashboardConfig:        newKeyBinding([]string{"c"}, "c", "打开配置"),
		Explore:                newKeyBinding([]string{"e"}, "e", "进入浏览"),
		Post:                   newKeyBinding([]string{"n"}, "n", "发布帖子"),
		Image:                  newKeyBinding([]string{"o"}, "o", "打开图片"),
		Praise:                 newKeyBinding([]string{"p"}, "p", "切换点赞"),
		Follow:                 newKeyBinding([]string{"f"}, "f", "切换关注"),
		Comment:                newKeyBinding([]string{"n"}, "n", "发表评论"),
		Reply:                  newKeyBinding([]string{"enter"}, "Enter", "引用评论"),
		Sort:                   newKeyBinding([]string{"s"}, "s", "切换排序"),
		Move:                   newKeyBinding([]string{"up", "down", "k", "j"}, "↑↓", "移动/滚动"),
		Page:                   newKeyBinding([]string{"pgup", "pgdown"}, "PgUp/PgDn", "快速翻页"),
		LeftRight:              newKeyBinding([]string{"left", "right"}, "←→", "切换/移动"),
		StartStop:              newKeyBinding([]string{"enter"}, "Enter", "启动/停止"),
		ModeNumbers:            newKeyBinding([]string{"1", "2", "3", "4"}, "1-4", "选择模式"),
		ModeCycle:              newKeyBinding([]string{"m"}, "m", "切换模式"),
		Backspace:              newKeyBinding([]string{"backspace"}, "Backspace", "删除字符"),
		GoToTop:                newKeyBinding([]string{"g"}, "g", "回到顶部"),
	},
}

var shortcut = ShortCut{
	Escape: newKeyBinding([]string{"esc"}, "Esc", "取消/关闭"),
	Enter:  newKeyBinding([]string{"enter"}, "Enter", "确认"),
	Save:   newKeyBinding([]string{"ctrl+s"}, "Ctrl+S", "保存/提交"),
	Resend: newKeyBinding([]string{"ctrl+r"}, "Ctrl+R", "重发"),

	Up:       newKeyBinding([]string{"up"}, "↑", "上移"),
	Down:     newKeyBinding([]string{"down"}, "↓", "下移"),
	Left:     newKeyBinding([]string{"left"}, "←", "左移"),
	Right:    newKeyBinding([]string{"right"}, "→", "右移"),
	PgUp:     newKeyBinding([]string{"pgup"}, "PgUp", "上翻"),
	PgDown:   newKeyBinding([]string{"pgdown"}, "PgDn", "下翻"),
	Tab:      newKeyBinding([]string{"tab"}, "Tab", "切换焦点"),
	ShiftTab: newKeyBinding([]string{"shift+tab"}, "Shift+Tab", "反向切换焦点"),

	VimUp:    newKeyBinding([]string{"k"}, "k", "上移"),
	VimDown:  newKeyBinding([]string{"j"}, "j", "下移"),
	VimLeft:  newKeyBinding([]string{"h"}, "h", "左移/返回"),
	VimRight: newKeyBinding([]string{"l"}, "l", "右移"),

	Increase: newKeyBinding([]string{"+", "="}, "+/-", "增加参数"),
	Decrease: newKeyBinding([]string{"-", "_"}, "+/-", "减少参数"),

	ToolConfig:      newKeyBinding([]string{"C"}, "C", "配置"),
	ToolLogs:        newKeyBinding([]string{"L"}, "L", "日志"),
	ToolInteractive: newKeyBinding([]string{"I"}, "I", "互动通知"),
	ToolSystem:      newKeyBinding([]string{"S"}, "S", "系统通知"),
	ToolHelp:        newKeyBinding([]string{"?"}, "?", "帮助"),

	MarkAllRead: newKeyBinding([]string{"a"}, "a", "全部已读"),
	Clear:       newKeyBinding([]string{"c"}, "c", "清除"),

	ConfigInsert:    newKeyBinding([]string{"i"}, "i", "插入"),
	ConfigAppend:    newKeyBinding([]string{"a"}, "a", "追加"),
	ConfigOpenBelow: newKeyBinding([]string{"o"}, "o", "下方新行"),
	ConfigOpenAbove: newKeyBinding([]string{"O"}, "O", "上方新行"),
	ConfigDelete:    newKeyBinding([]string{"x"}, "x", "删除字符"),
	ConfigLineStart: newKeyBinding([]string{"0"}, "0", "行首"),
	ConfigLineEnd:   newKeyBinding([]string{"$"}, "$", "行尾"),
	ConfigGo:        newKeyBinding([]string{"g"}, "gg", "文档顶部"),
	ConfigDocBottom: newKeyBinding([]string{"G"}, "G", "文档底部"),

	HomeModeCycle:     newKeyBinding([]string{"m"}, "m", "切换模式"),
	HomeFetchImages:   newKeyBinding([]string{"i"}, "i", "抓图开关"),
	HomeSaveJSON:      newKeyBinding([]string{"j"}, "j", "保存JSON"),
	HomeConvertWebp:   newKeyBinding([]string{"w"}, "w", "转换WebP"),
	HomeThumbnailPrev: newKeyBinding([]string{"["}, "[", "缩小缩略图起点"),
	HomeThumbnailNext: newKeyBinding([]string{"]"}, "]", "增大缩略图起点"),
}

func helpItemsFromBindings(bindings ...key.Binding) []helpItem {
	items := make([]helpItem, 0, len(bindings))
	for _, binding := range bindings {
		if !binding.Enabled() {
			continue
		}
		help := binding.Help()
		if help.Key == "" || help.Desc == "" {
			continue
		}
		items = append(items, helpItem{key: help.Key, desc: help.Desc})
	}
	return items
}

func (m Model) contextualHelpItems() []helpItem {
	bindings := []key.Binding{keymap.Direct.CloseHelp}
	globalTools := []key.Binding{
		keymap.Leader.Config,
		keymap.Direct.Logs,
		keymap.Direct.Notifications,
		keymap.Prefix.Command,
	}

	switch {
	case m.Dialog == DialogImage:
		bindings = append(bindings,
			newKeyBinding([]string{"left", "right"}, "←→", "切换图片"),
			newKeyBinding([]string{"esc"}, "Esc", "关闭图片"),
		)
	case m.Page == PageDashboard:
		bindings = append(bindings,
			keymap.Direct.Explore,
			keymap.Direct.DashboardNotifications,
			keymap.Direct.DashboardConfig,
			newKeyBinding([]string{"?"}, "?", "项目帮助"),
			keymap.Prefix.Command,
		)
	case m.Page == PageHome:
		bindings = append(bindings,
			keymap.Direct.Tab,
			keymap.Direct.ShiftTab,
			keymap.Direct.ModeNumbers,
			keymap.Direct.LeftRight,
			keymap.Direct.StartStop,
		)
		if m.Home.CrawlerState != CrawlerRunning {
			bindings = append(bindings, keymap.Direct.ModeCycle)
		}
		bindings = append(bindings, globalTools...)
	case m.Page == PageSchedule || m.Page == PageScores:
		bindings = append(bindings, keymap.Direct.Tab, keymap.Direct.ShiftTab, keymap.Direct.Refresh)
		if m.Page == PageSchedule {
			bindings = append(bindings,
				newKeyBinding([]string{"up", "down", "k", "j"}, "↑↓/kj", "选择行"),
				newKeyBinding([]string{"left", "right", "h", "l"}, "←→/hl", "选择列"),
				keymap.Direct.GoToTop,
				keymap.Direct.OpenDetail,
			)
		}
		if m.Page == PageScores {
			bindings = append(bindings,
				newKeyBinding([]string{"up", "down", "k", "j"}, "↑↓/kj", "滚动成绩"),
				newKeyBinding([]string{"pgup", "pgdown"}, "PgUp/PgDn", "成绩翻页"),
			)
		}
		bindings = append(bindings, globalTools...)
	case m.Posts.Searching:
		bindings = append(bindings,
			keymap.CommandInput.Submit,
			keymap.Direct.LeftRight,
			keymap.Direct.Backspace,
		)
	case m.Posts.ShowPostDetail:
		bindings = append(bindings,
			newKeyBinding([]string{"tab"}, "Tab", "切换正文/评论"),
			newKeyBinding([]string{"up", "down"}, "↑↓", "滚动当前区域"),
			keymap.Direct.Page,
			keymap.Direct.GoToTop,
			keymap.Direct.Sort,
			keymap.Direct.Refresh,
			keymap.Leader.Image,
		)
		if m.Posts.CanWrite {
			bindings = append(bindings,
				keymap.Direct.Praise,
				keymap.Direct.Follow,
				keymap.Direct.Comment,
				keymap.Direct.Reply,
				keymap.Leader.Praise,
				keymap.Leader.Follow,
				keymap.Leader.Comment,
			)
		}
	default:
		bindings = append(bindings,
			keymap.Direct.Tab,
			keymap.Direct.ShiftTab,
			newKeyBinding([]string{"up", "down"}, "↑↓", "选择帖子"),
			keymap.Direct.Page,
			keymap.Direct.GoToTop,
			keymap.Direct.OpenDetail,
			keymap.Direct.Search,
			newKeyBinding([]string{"r"}, "r", "刷新列表"),
		)
		if m.Session.Mode == SessionModeOnline {
			bindings = append(bindings, keymap.Direct.Tags)
		}
		if m.Posts.CanWrite {
			bindings = append(bindings,
				keymap.Direct.Post,
				keymap.Direct.Praise,
				keymap.Direct.Follow,
			)
		}
		bindings = append(bindings, globalTools...)
	}

	return helpItemsFromBindings(bindings...)
}

func (m Model) keyMode() KeyMode {
	switch m.Dialog {
	case DialogHelp:
		return KeyModeHelp
	case DialogTools:
		if m.ToolsDialog.Section() == ToolsSectionConfig {
			if m.ToolsDialog.Config.Mode() == ConfigEditorInsert {
				return KeyModeToolsConfigEdit
			}
			return KeyModeToolsConfig
		}
		return KeyModeTools
	case DialogImage:
		return KeyModeImage
	case DialogSessionPrompt:
		if m.SessionDialog.NeedsCredentials() {
			return KeyModeSessionInsert
		}
		return KeyModeSession
	case DialogAuthChallenge:
		return KeyModeAuthInsert
	case DialogComposer:
		return KeyModeComposerInsert
	case DialogTags:
		return KeyModeTags
	}

	if m.Page == PageDashboard {
		return KeyModeDashboard
	}
	if m.Page == PageHome {
		return KeyModeHome
	}
	if m.Page == PageSchedule {
		return KeyModeSchedule
	}
	if m.Page == PageScores {
		return KeyModeScores
	}
	if m.Posts.Searching {
		return KeyModePostsSearch
	}
	if m.Posts.ShowPostDetail {
		return KeyModePostsDetail
	}
	return KeyModePostsList
}

func (m Model) acceptsLeaderAndCommand() bool {
	switch m.keyMode() {
	case KeyModePostsSearch, KeyModeToolsConfigEdit, KeyModeSessionInsert, KeyModeAuthInsert, KeyModeComposerInsert:
		return false
	default:
		return true
	}
}

func (m Model) handleKeymapPrefix(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	if m.CommandActive {
		return m.handleCommandInput(msg)
	}
	if m.LeaderPending {
		return m.handleLeaderKey(msg)
	}
	if !m.acceptsLeaderAndCommand() {
		return m, nil, false
	}
	switch {
	case key.Matches(msg, keymap.Prefix.Leader):
		m.LeaderPending = true
		return m, nil, true
	case key.Matches(msg, keymap.Prefix.Command):
		m.CommandActive = true
		m.CommandInput = ""
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) handleDirectKey(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	if key.Matches(msg, keymap.Direct.Close) &&
		m.Dialog != DialogNone &&
		m.Dialog != DialogSessionPrompt &&
		m.Dialog != DialogAuthChallenge &&
		!(m.Dialog == DialogTools &&
			m.ToolsDialog.Section() == ToolsSectionConfig &&
			m.ToolsDialog.Config.Mode() == ConfigEditorInsert) {
		m.Dialog = DialogNone
		return m, nil, true
	}
	if key.Matches(msg, keymap.Direct.ToolsHelp) && m.Dialog == DialogHelp {
		m.Dialog = DialogNone
		return m, nil, true
	}
	if key.Matches(msg, keymap.Direct.Quit) && m.Dialog == DialogNone && !m.Posts.Searching {
		return m, tea.Quit, true
	}
	if key.Matches(msg, keymap.Direct.ToolsHelp) && m.Dialog == DialogNone && !m.Posts.Searching {
		if m.Page == PageDashboard {
			m.Dialog = DialogTools
			m.ToolsDialog.Switch(ToolsSectionHelp)
		} else {
			m.Dialog = DialogHelp
		}
		return m, nil, true
	}
	if m.Dialog == DialogNone && m.Page == PageDashboard {
		switch {
		case key.Matches(msg, keymap.Direct.Explore):
			next, cmd := m.enterDashboardExplore()
			return next, cmd, true
		case key.Matches(msg, keymap.Direct.DashboardNotifications):
			next, cmd := m.openNotificationsDialog(models.NotificationTypeInteractive)
			return next, cmd, true
		case key.Matches(msg, keymap.Direct.DashboardConfig):
			return m.openConfigDialog(), loadConfigCmd(), true
		}
	}
	if m.Dialog == DialogNone && !m.Posts.Searching && !m.Posts.ShowPostDetail {
		switch {
		case key.Matches(msg, keymap.Direct.Logs):
			next, cmd := m.openLogsDialog()
			return next, cmd, true
		case key.Matches(msg, keymap.Direct.Notifications):
			next, cmd := m.openNotificationsDialog(m.ToolsDialog.Notifications.MessageType())
			return next, cmd, true
		}
	}
	if (key.Matches(msg, keymap.Direct.Tab) || key.Matches(msg, keymap.Direct.ShiftTab)) && m.Dialog == DialogNone && !m.Posts.Searching && !m.Posts.ShowPostDetail {
		if key.Matches(msg, keymap.Direct.ShiftTab) {
			next, cmd := m.switchPrevPage()
			return next, cmd, true
		}
		next, cmd := m.switchNextPage()
		return next, cmd, true
	}
	return m, nil, false
}

func (m Model) switchNextPage() (Model, tea.Cmd) {
	m.TabCursor = (m.TabCursor + 1) % pageCount
	return m.switchPage(Page(m.TabCursor))
}

func (m Model) switchPrevPage() (Model, tea.Cmd) {
	m.TabCursor = (m.TabCursor - 1 + pageCount) % pageCount
	return m.switchPage(Page(m.TabCursor))
}

func (m Model) handleCommandInput(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, keymap.CommandInput.Cancel):
		m.CommandActive = false
		m.CommandInput = ""
		return m, nil, true
	case key.Matches(msg, keymap.CommandInput.Submit):
		input := strings.TrimSpace(m.CommandInput)
		m.CommandActive = false
		m.CommandInput = ""
		next, cmd := m.executeCommand(input)
		return next, cmd, true
	case key.Matches(msg, keymap.CommandInput.Delete):
		runes := []rune(m.CommandInput)
		if len(runes) > 0 {
			m.CommandInput = string(runes[:len(runes)-1])
		}
		return m, nil, true
	default:
		if msg.Text != "" {
			m.CommandInput += msg.Text
			return m, nil, true
		}
		return m, nil, true
	}
}

func (m Model) handleLeaderKey(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	m.LeaderPending = false
	if key.Matches(msg, keymap.CommandInput.Cancel) {
		return m, nil, true
	}
	switch {
	case key.Matches(msg, keymap.Leader.Help):
		m.Dialog = DialogTools
		m.ToolsDialog.Switch(ToolsSectionHelp)
		return m, nil, true
	case key.Matches(msg, keymap.Leader.Config):
		return m.openConfigDialog(), loadConfigCmd(), true
	case key.Matches(msg, keymap.Leader.Logs):
		next, cmd := m.openLogsDialog()
		return next, cmd, true
	case key.Matches(msg, keymap.Leader.Notifications):
		next, cmd := m.openNotificationsDialog(models.NotificationTypeInteractive)
		return next, cmd, true
	case key.Matches(msg, keymap.Leader.Quit):
		if m.Dialog != DialogNone {
			m.Dialog = DialogNone
			return m, nil, true
		}
		if m.Posts.ShowPostDetail {
			next, cmd := m.handlePostsKey(keyByString("esc"))
			return next, cmd, true
		}
		return m, tea.Quit, true
	case key.Matches(msg, keymap.Leader.Praise):
		return m.handlePostsActionKey("p")
	case key.Matches(msg, keymap.Leader.Follow):
		return m.handlePostsActionKey("f")
	case key.Matches(msg, keymap.Leader.Image):
		return m.handlePostsActionKey("o")
	case key.Matches(msg, keymap.Leader.Tags):
		return m.handlePostsActionKey("t")
	case key.Matches(msg, keymap.Leader.Post):
		return m.handlePostsActionKey("n")
	case key.Matches(msg, keymap.Leader.Comment):
		return m.handlePostsActionKey("n")
	case key.Matches(msg, keymap.Leader.Home):
		next, cmd := m.switchPage(PageHome)
		return next, cmd, true
	case key.Matches(msg, keymap.Leader.PageHome):
		next, cmd := m.switchPage(PageHome)
		return next, cmd, true
	case key.Matches(msg, keymap.Leader.PagePosts):
		next, cmd := m.switchPage(PagePosts)
		return next, cmd, true
	case key.Matches(msg, keymap.Leader.PageSchedule):
		next, cmd := m.switchPage(PageSchedule)
		return next, cmd, true
	case key.Matches(msg, keymap.Leader.PageScores):
		next, cmd := m.switchPage(PageScores)
		return next, cmd, true
	default:
		return m, m.showToast("未知快捷键: Space " + msg.String()), true
	}
}

func (m Model) executeCommand(input string) (Model, tea.Cmd) {
	if input == "" {
		return m, nil
	}
	fields := strings.Fields(input)
	name := strings.ToLower(fields[0])
	arg := strings.TrimSpace(strings.TrimPrefix(input, fields[0]))

	switch name {
	case "help":
		m.Dialog = DialogHelp
		return m, nil
	case "q", "quit":
		return m, tea.Quit
	case "config":
		return m.openConfigDialog(), loadConfigCmd()
	case "logs":
		return m.openLogsDialog()
	case "notifications", "noti":
		return m.openNotificationsDialog(models.NotificationTypeInteractive)
	case "home":
		return m.switchPage(PageHome)
	case "posts":
		return m.switchPage(PagePosts)
	case "schedule":
		return m.switchPage(PageSchedule)
	case "scores":
		return m.switchPage(PageScores)
	case "reload", "refresh":
		return m.reloadCurrentMode()
	case "search":
		return m.commandSearch(arg)
	case "tag":
		return m.commandTags()
	case "clear":
		if m.Page == PagePosts {
			return m.clearActiveFilters()
		}
		return m, nil
	case "post":
		return m.commandPostsAction("n")
	case "comment":
		return m.commandPostsAction("n")
	case "reply":
		return m.commandPostsAction("enter")
	case "image":
		return m.commandPostsAction("o")
	case "offline":
		offlineCmd := m.forceOfflineMode(m.Session.Message)
		m.Dialog = DialogNone
		m.Posts.PostListLoading = true
		return m, tea.Batch(offlineCmd, loadPostsCmd(m.Provider, 0, m.Posts.PostPerPage, 0))
	default:
		return m, m.showToast("未知命令: :" + input)
	}
}

func (m Model) openConfigDialog() Model {
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionConfig)
	m.ToolsDialog.Config = NewConfigDialog(m.Config)
	return m
}

func (m Model) openLogsDialog() (Model, tea.Cmd) {
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionLogs)
	m.ToolsDialog.Logs.SetLoading(true)
	return m, loadLogsCmd()
}

func (m Model) openNotificationsDialog(messageType models.NotificationType) (Model, tea.Cmd) {
	m.Dialog = DialogTools
	m.ToolsDialog.Switch(ToolsSectionInteractive)
	m.ToolsDialog.Notifications = NewNotificationDialog()
	m.ToolsDialog.Notifications.SetMessageType(messageType)
	m.ToolsDialog.Notifications.SetLoading(true)
	return m, loadNotificationsCmd(m.Client, messageType)
}

func (m Model) switchPage(page Page) (Model, tea.Cmd) {
	m.Dialog = DialogNone
	m.Page = page
	m.TabCursor = int(page)
	switch page {
	case PagePosts:
		if len(m.Posts.PostList) == 0 {
			m.Posts.PostListLoading = true
			return m, loadPostsCmd(m.Provider, 0, m.Posts.PostPerPage, m.Posts.ActiveTagID)
		}
	case PageSchedule:
		if len(m.Schedule.Rows) == 0 && m.Schedule.Error == "" {
			m.Schedule.Loading = true
			return m, loadCourseScheduleCmd(m.Provider)
		}
	case PageScores:
		if m.Scores.Summary == nil && m.Scores.Error == "" {
			m.Scores.Loading = true
			return m, loadScoresCmd(m.Provider)
		}
	}
	m.syncPostsPage()
	return m, nil
}

func (m Model) reloadCurrentMode() (Model, tea.Cmd) {
	switch m.keyMode() {
	case KeyModeTools:
		switch m.ToolsDialog.Section() {
		case ToolsSectionLogs:
			return m.handleToolsDialogKey(keyByString("r"))
		case ToolsSectionInteractive, ToolsSectionSystem:
			return m.handleToolsDialogKey(keyByString("r"))
		}
	case KeyModeSchedule:
		return m.handleScheduleKey(keyByString("r"))
	case KeyModeScores:
		return m.handleScoresKey(keyByString("r"))
	case KeyModePostsList, KeyModePostsDetail:
		return m.handlePostsKey(keyByString("r"))
	}
	return m, nil
}

func (m Model) commandSearch(query string) (Model, tea.Cmd) {
	if m.Page != PagePosts {
		next, cmd := m.switchPage(PagePosts)
		m = next
		if query == "" {
			return m.handlePostsKey(keyByString("/"))
		}
		_ = cmd
	}
	if query == "" {
		return m.handlePostsKey(keyByString("/"))
	}
	m.Posts.Searching = false
	m.Posts.SearchInput = query
	m.Posts.SearchField = newSearchInput()
	m.Posts.SearchField.SetValue(query)
	m.Posts.SearchHistory = appendSearchHistory(query)
	m.Posts.SearchHistoryIndex = len(m.Posts.SearchHistory)
	m.Posts.SearchHistoryDraft = ""
	m.Posts.PostListLoading = true
	m.Posts.PostsMode = PostsModeSearchInput
	return m, searchPostsCmd(m.Provider, query, 0, m.Posts.PostPerPage, m.Posts.ActiveTagID)
}

func (m Model) commandTags() (Model, tea.Cmd) {
	if m.Page != PagePosts {
		var cmd tea.Cmd
		m, cmd = m.switchPage(PagePosts)
		_ = cmd
	}
	return m.handlePostsKey(keyByString("t"))
}

func (m Model) commandPostsAction(key string) (Model, tea.Cmd) {
	if m.Page != PagePosts || m.Dialog != DialogNone {
		return m, nil
	}
	return m.handlePostsKey(keyByString(key))
}

func (m Model) handlePostsActionKey(key string) (Model, tea.Cmd, bool) {
	next, cmd := m.commandPostsAction(key)
	return next, cmd, true
}

func keyByString(key string) tea.KeyPressMsg {
	switch key {
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	default:
		runes := []rune(key)
		if len(runes) == 1 {
			return tea.KeyPressMsg{Code: runes[0], Text: key}
		}
		return tea.KeyPressMsg{Text: key}
	}
}
