package tui

import (
	"time"

	"treehole/internal/client"
	"treehole/internal/config"
	"treehole/internal/db"
	"treehole/internal/models"

	tea "github.com/charmbracelet/bubbletea"
)

type Page int

const (
	PageHome Page = iota
	PagePosts
)

type DialogType int

const (
	DialogNone DialogType = iota
	DialogConfig
	DialogLogs
	DialogHelp
	DialogSessionPrompt
	DialogAuthChallenge
	DialogComposer
	DialogTags
)

type CrawlerState int

const (
	CrawlerStopped CrawlerState = iota
	CrawlerRunning
	CrawlerError
)

type HomeFocus int

const (
	HomeFocusStart HomeFocus = iota
	HomeFocusStop
	HomeFocusMode
)

type PostsMode int

const (
	PostsModeList PostsMode = iota
	PostsModeSearchInput
	PostsModeSearchResults
	PostsModeDetail
)

type DetailFocus int

const (
	DetailFocusPost DetailFocus = iota
	DetailFocusComments
)

type CrawlMsg struct {
	Page     int
	Duration time.Duration
	Error    error
}

type TickMsg time.Time

type LoginMsg struct {
	Username string
	Error    error
}

type LoadPostsMsg struct {
	Posts         []models.Post
	RequestCursor int
	NextCursor    int
	HasMore       bool
	Error         error
}

type LoadCommentsMsg struct {
	Comments      []models.Comment
	RequestCursor int32
	NextCursor    int32
	HasMore       bool
	SortAsc       bool
	Error         error
}

type SearchPostsMsg struct {
	Posts         []models.Post
	RequestCursor int
	NextCursor    int
	HasMore       bool
	Error         error
}

type LoadPostDetailMsg struct {
	Post       *models.Post
	Comments   []models.Comment
	NextCursor int32
	HasMore    bool
	SortAsc    bool
	Error      error
}

type LoadLogsMsg struct {
	Lines []string
	Error error
}

type SessionRefreshMsg struct {
	State SessionState
	Error error
}

type AuthChallengeResultMsg struct {
	State SessionState
	Error error
}

type AuthSMSSentMsg struct {
	Error   error
	Message string
}

type ActionResultMsg struct {
	Kind    string
	Message string
	Post    *models.Post
	Error   error
}

type LoadTagsMsg struct {
	Tags  []models.Tag
	Error error
}

type LoadConfigMsg struct {
	Config *config.Config
	Error  error
}

type SaveConfigMsg struct {
	Error error
}

type CrawlMode int

const (
	CrawlSequential CrawlMode = iota
	CrawlMonitor
)

type SessionMode int

const (
	SessionModeOffline SessionMode = iota
	SessionModeOnline
)

type SessionFailureReason int

const (
	SessionFailureReasonNone SessionFailureReason = iota
	SessionFailureReasonLogin
	SessionFailureReasonNetwork
)

type AuthChallengeType int

const (
	AuthChallengeTypeNone AuthChallengeType = iota
	AuthChallengeTypeUsername
	AuthChallengeTypePassword
	AuthChallengeTypeSMS
	AuthChallengeTypeOTP
)

type SessionState struct {
	Mode               SessionMode
	HasSession         bool
	CanReadOnline      bool
	CanWriteOnline     bool
	FailureReason      SessionFailureReason
	Message            string
	LastFallbackReason string
	NeedsConfig        bool
	Challenge          AuthChallengeType
	ChallengeMessage   string
}

type Model struct {
	Page      Page
	Width     int
	Height    int
	TabCursor int

	Dialog DialogType

	Home     HomePageModel
	Database *db.Database
	Client   *client.Client
	Config   *config.Config
	Provider PostsProvider
	Session  SessionState

	Posts PostsPageModel

	ConfigDialog  ConfigDialogModel
	LogsDialog    LogsDialogModel
	SessionDialog SessionPromptDialogModel
	AuthDialog    AuthChallengeDialogModel
	Composer      ComposerDialogModel
	TagsDialog    TagsDialogModel

	LastError string
	Capture   *CaptureSink
}

func NewModel(database *db.Database, client *client.Client, cfg *config.Config, session SessionState) Model {
	applyTheme("")

	var provider PostsProvider = NewOfflinePostsProvider(database)
	if session.CanReadOnline {
		session.Mode = SessionModeOnline
		provider = NewOnlinePostsProvider(client)
	} else {
		session.Mode = SessionModeOffline
	}

	dialog := DialogNone
	sessionDialog := NewSessionPromptDialog(session)
	authDialog := NewAuthChallengeDialog(session)
	if session.Challenge != AuthChallengeTypeNone {
		dialog = DialogAuthChallenge
	} else if session.FailureReason != SessionFailureReasonNone {
		dialog = DialogSessionPrompt
	}

	return Model{
		Page:          PagePosts,
		TabCursor:     1,
		Dialog:        dialog,
		Home:          NewHomePageModel(),
		Database:      database,
		Client:        client,
		Config:        cfg,
		Provider:      provider,
		Session:       session,
		Posts:         NewPostsPageModel(),
		ConfigDialog:  NewConfigDialog(cfg),
		LogsDialog:    NewLogsDialog(),
		SessionDialog: sessionDialog,
		AuthDialog:    authDialog,
		Composer:      NewComposerDialog(),
		TagsDialog:    NewTagsDialog(),
	}
}

func (m Model) Init() tea.Cmd {
	username := ""
	if m.Config != nil {
		username = m.Config.Username
	}
	return tea.Batch(
		func() tea.Msg {
			return LoginMsg{Username: username}
		},
		loadPostsCmd(m.Provider, 0, m.Posts.PostPerPage, m.Posts.ActiveTagID),
		tickCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (m *Model) ensureDialogModels() {
	if !m.ConfigDialog.initialized() {
		m.ConfigDialog = NewConfigDialog(m.Config)
	}
	if !m.LogsDialog.initialized() {
		m.LogsDialog = NewLogsDialog()
	}
	if !m.SessionDialog.initialized() {
		m.SessionDialog = NewSessionPromptDialog(m.Session)
	}
	if !m.AuthDialog.initialized() {
		m.AuthDialog = NewAuthChallengeDialog(m.Session)
	}
	if !m.Composer.initialized() {
		m.Composer = NewComposerDialog()
	}
	if !m.TagsDialog.initialized() {
		m.TagsDialog = NewTagsDialog()
	}
	m.Posts.ensureInitialized()
}

func (m Model) calcPostViewportHeight() int {
	return m.Posts.calcPostViewportHeight(m.Height)
}
