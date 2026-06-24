package tui

import (
	"time"

	"treehole/internal/client"
	"treehole/internal/config"
	"treehole/internal/db"
	"treehole/internal/models"

	tea "charm.land/bubbletea/v2"
)

type Page int

const (
	PageHome Page = iota
	PagePosts
	PageSchedule
	PageScores
	PageDashboard
)

const pageCount = int(PageScores) + 1

type DialogType int

const (
	DialogNone DialogType = iota
	DialogTools
	DialogImage
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
	Summary  string
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

type LoadNotificationsMsg struct {
	MessageType models.NotificationType
	Items       []models.Notification
	Total       int
	Error       error
}

type LoadDashboardNotificationsMsg struct {
	Items []models.Notification
	Error error
}

type NotificationActionMsg struct {
	MessageType models.NotificationType
	ID          int
	All         bool
	Error       error
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

type LoadCourseScheduleMsg struct {
	Rows  []models.CourseScheduleRow
	Error error
}

type LoadScoresMsg struct {
	Summary *models.ScoreSummary
	Error   error
}

type LoadConfigMsg struct {
	Config *config.Config
	Error  error
}

type SaveConfigMsg struct {
	Config *config.Config
	Error  error
}

type CrawlMode int

const (
	CrawlSequential CrawlMode = iota
	CrawlMonitor
	CrawlFetchImages
	CrawlFetchThumbnails
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

	Posts     PostsPageModel
	Schedule  CourseSchedulePageModel
	Scores    ScorePageModel
	Dashboard DashboardModel

	ToolsDialog   ToolsDialogModel
	ImageDialog   ImageDialogModel
	SessionDialog SessionPromptDialogModel
	AuthDialog    AuthChallengeDialogModel
	Composer      ComposerDialogModel
	TagsDialog    TagsDialogModel

	LastError string
	Capture   *CaptureSink
	Images    *KittyImageRenderer

	ToastMsg       string
	ToastExpiresAt time.Time
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

	sessionDialog := NewSessionPromptDialog(session)
	authDialog := NewAuthChallengeDialog(session)

	return Model{
		Page:          PageDashboard,
		TabCursor:     1,
		Dialog:        DialogNone,
		Home:          NewHomePageModel(),
		Database:      database,
		Client:        client,
		Config:        cfg,
		Provider:      provider,
		Session:       session,
		Posts:         NewPostsPageModel(),
		Dashboard:     NewDashboardModel(),
		ToolsDialog:   NewToolsDialog(cfg),
		ImageDialog:   NewImageDialog(),
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
		loadDashboardNotificationsCmd(m.Client),
		tickCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (m *Model) ensureDialogModels() {
	if !m.ToolsDialog.initialized() {
		m.ToolsDialog = NewToolsDialog(m.Config)
	}
	if !m.ImageDialog.initialized() {
		m.ImageDialog = NewImageDialog()
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
	if m.Posts.ImageClient == nil {
		m.Posts.ImageClient = m.Client
	}
	m.Posts.ensureInitialized()
}

func (m Model) calcPostViewportHeight() int {
	return m.Posts.calcPostViewportHeight(m.Height)
}

func (m Model) imageRefreshCmd(cmd tea.Cmd) tea.Cmd {
	return cmd
}
