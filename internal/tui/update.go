package tui

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"treehole/internal/client"
	"treehole/internal/config"
	"treehole/internal/crawler"
	"treehole/internal/db"
	"treehole/internal/models"

	tea "charm.land/bubbletea/v2"
)

type clearToastMsg struct{}

const toastDuration = 3 * time.Second

func (m *Model) showToast(text string) tea.Cmd {
	m.ToastMsg = text
	m.ToastExpiresAt = time.Now().Add(toastDuration)
	return tea.Tick(toastDuration, func(t time.Time) tea.Msg {
		return clearToastMsg{}
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.ensureDialogModels()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.syncPostsPage()
		return m, m.imageRefreshCmd(nil)

	case clearToastMsg:
		if !m.ToastExpiresAt.IsZero() && time.Now().After(m.ToastExpiresAt) {
			m.ToastMsg = ""
			m.ToastExpiresAt = time.Time{}
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case TickMsg:
		return m, tickCmd()

	case LoginMsg:
		if msg.Error != nil {
			m.LastError = msg.Error.Error()
			m.Home.LoggedIn = false
		} else {
			m.Home.LoggedIn = true
			m.Home.LoginUser = msg.Username
		}
		return m, nil

	case CrawlMsg:
		if msg.Error != nil {
			log.Printf("[Crawler] 爬虫错误: 第 %d 页, %v", msg.Page, msg.Error)
			m.Home.CrawlerState = CrawlerError
			m.Home.HomeLastError = msg.Error.Error()
		} else {
			m.Home.LastCrawlPage = msg.Page
			m.Home.LastCrawlTime = msg.Duration
			m.Home.LastCrawlSummary = msg.Summary
			if m.Home.CrawlerState == CrawlerRunning {
				if m.Home.CrawlMode == CrawlMonitor {
					return m, crawlMonitorWithOptionsCmd(m.Client, m.Database, m.Home)
				}
				if m.Home.CrawlMode == CrawlSequential {
					return m, crawlPageWithOptionsCmd(m.Client, m.Database, msg.Page+1, m.Home)
				}
				m.Home.CrawlerState = CrawlerStopped
			}
		}
		return m, nil

	case LoadPostsMsg:
		m.Posts.PostListLoading = false
		if msg.Error != nil {
			m.Posts.PostListError = msg.Error.Error()
			m.handleOnlineReadFailure(msg.Error)
		} else {
			m.Posts.PostListError = ""
			if !m.Posts.SearchActive {
				if msg.RequestCursor == 0 {
					m.Posts.PostList = msg.Posts
					m.Posts.SelectedPostIdx = 0
					m.Posts.CursorLine = 0
					m.Posts.PostViewport.GotoTop()
					m.Posts.PostListTotal = len(m.Posts.PostList)
					m.Posts.PostListCursor = msg.NextCursor
					m.Posts.PostListHasMore = msg.HasMore
				} else {
					var deduped []models.Post
					if len(m.Posts.PostList) > 0 {
						lastTs := m.Posts.PostList[len(m.Posts.PostList)-1].Timestamp
						for _, p := range msg.Posts {
							if p.Timestamp < lastTs {
								deduped = append(deduped, p)
							}
						}
					} else {
						deduped = msg.Posts
					}
					if len(deduped) > 0 {
						m.Posts.PostList = append(m.Posts.PostList, deduped...)
						m.Posts.PostListTotal = len(m.Posts.PostList)
						m.Posts.PostListCursor = msg.NextCursor
						m.Posts.PostListHasMore = msg.HasMore
					} else if msg.HasMore {
						m.Posts.PostListLoading = true
						m.Posts.PostListCursor = msg.NextCursor
						m.syncPostsPage()
						return m, m.imageRefreshCmd(loadPostsCmd(m.Provider, msg.NextCursor, m.Posts.PostPerPage, m.Posts.ActiveTagID))
					} else {
						m.Posts.PostListCursor = msg.NextCursor
						m.Posts.PostListHasMore = false
					}
				}
				m.Posts.PostsMode = PostsModeList
			}
		}
		m.syncPostsPage()
		return m, m.imageRefreshCmd(nil)

	case LoadCommentsMsg:
		m.Posts.CommentListLoading = false
		if msg.Error != nil {
			m.Posts.CommentListError = msg.Error.Error()
			m.handleOnlineReadFailure(msg.Error)
		} else {
			m.Posts.CommentListError = ""
			if msg.RequestCursor == 0 {
				m.Posts.CommentList = msg.Comments
				m.Posts.CommentCursorLine = 0
				m.Posts.SelectedCommentIdx = 0
				m.Posts.CommentViewport.GotoTop()
			} else {
				m.Posts.CommentList = append(m.Posts.CommentList, msg.Comments...)
			}
			m.Posts.CommentListCursor = msg.NextCursor
			m.Posts.CommentListHasMore = msg.HasMore
			m.Posts.CommentSortAsc = msg.SortAsc
		}
		m.syncPostsPage()
		return m, m.imageRefreshCmd(nil)

	case SearchPostsMsg:
		m.Posts.PostListLoading = false
		if msg.Error != nil {
			m.Posts.PostListError = msg.Error.Error()
			m.handleOnlineReadFailure(msg.Error)
		} else {
			m.Posts.PostListError = ""
			if msg.RequestCursor == 0 {
				m.Posts.PostList = msg.Posts
				m.Posts.SelectedPostIdx = 0
				m.Posts.CursorLine = 0
				m.Posts.PostViewport.GotoTop()
			} else if m.Posts.SearchActive {
				m.Posts.PostList = append(m.Posts.PostList, msg.Posts...)
			}
			m.Posts.PostListTotal = len(m.Posts.PostList)
			m.Posts.PostListCursor = msg.NextCursor
			m.Posts.PostListHasMore = msg.HasMore
			m.Posts.SearchActive = true
			m.Posts.Searching = false
			m.Posts.PostsMode = PostsModeSearchResults
		}
		m.syncPostsPage()
		return m, m.imageRefreshCmd(nil)

	case LoadPostDetailMsg:
		m.Posts.CommentListLoading = false
		if msg.Error != nil {
			m.Posts.CommentListError = msg.Error.Error()
			m.handleOnlineReadFailure(msg.Error)
		} else {
			m.Posts.CommentListError = ""
			m.Posts.CurrentPost = msg.Post
			m.Posts.CommentList = msg.Comments
			m.Posts.CommentCursorLine = 0
			m.Posts.SelectedCommentIdx = 0
			m.Posts.CommentListCursor = msg.NextCursor
			m.Posts.CommentListHasMore = msg.HasMore
			m.Posts.CommentSortAsc = msg.SortAsc
			m.Posts.commentContent = ""
			m.Posts.postBodyContent = ""
			m.Posts.PostBodyViewport.GotoTop()
			m.Posts.CommentViewport.GotoTop()
		}
		m.syncPostsPage()
		return m, nil

	case LoadLogsMsg:
		if msg.Error != nil {
			m.ToolsDialog.Logs.SetError(msg.Error)
		} else {
			m.ToolsDialog.Logs.SetLines(msg.Lines)
		}
		return m, nil

	case LoadNotificationsMsg:
		if msg.Error != nil {
			m.ToolsDialog.Notifications.SetError(msg.Error)
			m.handleOnlineReadFailure(msg.Error)
		} else if msg.MessageType == m.ToolsDialog.Notifications.MessageType() {
			m.ToolsDialog.Notifications.SetNotifications(msg.MessageType, msg.Items, msg.Total)
		}
		return m, nil

	case LoadDashboardNotificationsMsg:
		m.Dashboard.SetNotifications(msg.Items, msg.Error)
		return m, nil

	case NotificationActionMsg:
		if msg.Error != nil {
			m.ToolsDialog.Notifications.SetError(msg.Error)
			m.LastError = msg.Error.Error()
			return m, nil
		}
		if msg.MessageType != m.ToolsDialog.Notifications.MessageType() {
			return m, nil
		}
		if msg.All {
			m.ToolsDialog.Notifications.MarkAllRead()
			m.Dashboard.MarkAllRead(msg.MessageType)
			return m, m.showToast("当前分类通知已全部设为已读")
		}
		m.ToolsDialog.Notifications.MarkRead(msg.ID)
		m.Dashboard.MarkRead(msg.ID)
		return m, m.showToast("通知已设为已读")

	case LoadConfigMsg:
		if msg.Error == nil && msg.Config != nil {
			m.Config = msg.Config
			m.ToolsDialog.Config.SetConfig(msg.Config)
		}
		return m, nil

	case SaveConfigMsg:
		if msg.Error != nil {
			m.LastError = msg.Error.Error()
			m.ToolsDialog.Config.SetSaveResult(msg.Error)
		} else {
			if msg.Config != nil {
				m.Config = msg.Config
			}
			m.ToolsDialog.Config.SetSaveResult(nil)
		}
		return m, nil

	case SessionRefreshMsg:
		sessionCmd := m.applySessionState(msg.State)
		if msg.State.Challenge != AuthChallengeTypeNone {
			m.AuthDialog.ApplyState(msg.State)
			m.Dialog = DialogAuthChallenge
		} else if msg.State.FailureReason != SessionFailureReasonNone {
			m.SessionDialog.ApplyState(msg.State)
			m.Dialog = DialogSessionPrompt
		} else if m.Dialog == DialogSessionPrompt || m.Dialog == DialogAuthChallenge {
			m.Dialog = DialogNone
		}
		if msg.Error == nil && msg.State.CanReadOnline {
			m.Posts.resetList()
			m.Posts.PostListLoading = true
			return m, tea.Batch(sessionCmd, loadPostsCmd(m.Provider, 0, m.Posts.PostPerPage, m.Posts.ActiveTagID))
		}
		return m, sessionCmd

	case AuthChallengeResultMsg:
		m.AuthDialog.SetSubmitting(false)
		if msg.Error != nil {
			m.AuthDialog.SetError(msg.Error)
			return m, nil
		}
		sessionCmd := m.applySessionState(msg.State)
		if msg.State.CanReadOnline {
			m.Dialog = DialogNone
			m.Posts.resetList()
			m.Posts.PostListLoading = true
			return m, tea.Batch(sessionCmd, loadPostsCmd(m.Provider, 0, m.Posts.PostPerPage, m.Posts.ActiveTagID))
		}
		m.AuthDialog.ApplyState(msg.State)
		m.Dialog = DialogAuthChallenge
		return m, sessionCmd

	case AuthSMSSentMsg:
		m.AuthDialog.SetSubmitting(false)
		if msg.Error != nil {
			m.AuthDialog.SetError(msg.Error)
		} else {
			m.AuthDialog.SetError(nil)
			m.AuthDialog.SetStatus(msg.Message)
		}
		return m, nil

	case ActionResultMsg:
		if msg.Error != nil {
			m.LastError = msg.Error.Error()
			if m.Dialog == DialogComposer {
				m.Composer.SetError(msg.Error)
				return m, nil
			}
		} else {
			m.LastError = ""
			m.Dialog = DialogNone
			toastCmd := m.showToast(msg.Message)
			if msg.Post != nil {
				m.Posts.updatePost(msg.Post)
				if m.Posts.CurrentPost != nil && m.Posts.CurrentPost.Pid == msg.Post.Pid {
					m.Posts.CurrentPost = msg.Post
				}
			}
			if msg.Kind == "post" {
				m.Posts.resetList()
				m.Posts.PostListLoading = true
				return m, tea.Batch(toastCmd, loadPostsCmd(m.Provider, 0, m.Posts.PostPerPage, m.Posts.ActiveTagID))
			}
			if m.Posts.CurrentPost != nil {
				m.Posts.CommentListLoading = true
				return m, tea.Batch(toastCmd, loadPostDetailCmd(m.Provider, m.Posts.CurrentPost.Pid, m.Posts.CommentSortAsc))
			}
			return m, toastCmd
		}

	case LoadTagsMsg:
		if msg.Error != nil {
			m.TagsDialog.SetError(msg.Error)
			m.LastError = msg.Error.Error()
		} else {
			m.TagsDialog.SetTags(msg.Tags)
		}
		return m, nil

	case LoadCourseScheduleMsg:
		m.Schedule.Loading = false
		if msg.Error != nil {
			m.Schedule.Error = msg.Error.Error()
			m.handleOnlineReadFailure(msg.Error)
		} else {
			m.Schedule.Error = ""
			m.Schedule.Rows = msg.Rows
		}
		return m, nil

	case LoadScoresMsg:
		m.Scores.Loading = false
		if msg.Error != nil {
			m.Scores.Error = msg.Error.Error()
			m.handleOnlineReadFailure(msg.Error)
		} else {
			m.Scores.Error = ""
			m.Scores.Summary = msg.Summary
			m.Scores.Offset = 0
		}
		return m, nil
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" &&
		m.Dialog != DialogNone &&
		m.Dialog != DialogSessionPrompt &&
		m.Dialog != DialogAuthChallenge &&
		!(m.Dialog == DialogTools &&
			m.ToolsDialog.Section() == ToolsSectionConfig &&
			m.ToolsDialog.Config.Mode() == ConfigEditorInsert) {
		m.Dialog = DialogNone
		return m, nil
	}
	if msg.String() == "h" && m.Dialog == DialogHelp {
		m.Dialog = DialogNone
		return m, nil
	}

	if msg.String() == "ctrl+q" && m.Dialog == DialogNone && !m.Posts.Searching {
		return m, tea.Quit
	}

	if msg.String() == "h" && m.Dialog == DialogNone && !m.Posts.Searching {
		m.Dialog = DialogHelp
		return m, nil
	}

	if m.Dialog == DialogNone && m.Page == PageDashboard {
		switch msg.String() {
		case "e":
			m.Page = PagePosts
			m.TabCursor = int(PagePosts)
			m.Posts.PostListLoading = true
			return m, loadPostsCmd(m.Provider, 0, m.Posts.PostPerPage, m.Posts.ActiveTagID)
		case "n":
			m.Dialog = DialogTools
			m.ToolsDialog.Switch(ToolsSectionInteractive)
			m.ToolsDialog.Notifications = NewNotificationDialog()
			m.ToolsDialog.Notifications.SetLoading(true)
			return m, loadNotificationsCmd(m.Client, models.NotificationTypeInteractive)
		}
	}

	if m.Dialog == DialogNone && !m.Posts.Searching && !m.Posts.ShowPostDetail {
		switch msg.String() {
		case "c":
			m.Dialog = DialogTools
			m.ToolsDialog.Switch(ToolsSectionConfig)
			m.ToolsDialog.Config = NewConfigDialog(m.Config)
			return m, loadConfigCmd()
		case "l":
			m.Dialog = DialogTools
			m.ToolsDialog.Switch(ToolsSectionLogs)
			m.ToolsDialog.Logs.SetLoading(true)
			return m, loadLogsCmd()
		case "b":
			m.Dialog = DialogTools
			m.ToolsDialog.Switch(ToolsSectionInteractive)
			m.ToolsDialog.Notifications = NewNotificationDialog()
			m.ToolsDialog.Notifications.SetLoading(true)
			return m, loadNotificationsCmd(m.Client, m.ToolsDialog.Notifications.MessageType())
		}
	}

	if msg.String() == "tab" && m.Dialog == DialogNone && !m.Posts.Searching && !m.Posts.ShowPostDetail {
		m.TabCursor = (m.TabCursor + 1) % pageCount
		m.Page = Page(m.TabCursor)
		if m.Page == PagePosts && len(m.Posts.PostList) == 0 {
			m.Posts.PostListLoading = true
			return m, loadPostsCmd(m.Provider, 0, m.Posts.PostPerPage, m.Posts.ActiveTagID)
		}
		if m.Page == PageSchedule && len(m.Schedule.Rows) == 0 && m.Schedule.Error == "" {
			m.Schedule.Loading = true
			return m, loadCourseScheduleCmd(m.Provider)
		}
		if m.Page == PageScores && m.Scores.Summary == nil && m.Scores.Error == "" {
			m.Scores.Loading = true
			return m, loadScoresCmd(m.Provider)
		}
		m.syncPostsPage()
		return m, nil
	}

	if m.Dialog != DialogNone {
		switch m.Dialog {
		case DialogTools:
			return m.handleToolsDialogKey(msg)
		case DialogImage:
			return m.handleImageDialogKey(msg)
		case DialogHelp:
			return m, nil
		case DialogSessionPrompt:
			return m.handleSessionDialogKey(msg)
		case DialogAuthChallenge:
			return m.handleAuthChallengeKey(msg)
		case DialogComposer:
			return m.handleComposerKey(msg)
		case DialogTags:
			return m.handleTagsDialogKey(msg)
		}
	}

	switch m.Page {
	case PageDashboard:
		return m, nil
	case PageHome:
		return m.handleHomeKey(msg)
	case PagePosts:
		return m.handlePostsKey(msg)
	case PageSchedule:
		return m.handleScheduleKey(msg)
	case PageScores:
		return m.handleScoresKey(msg)
	}
	return m, nil
}

func (m Model) handleHomeKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	action := m.Home.Update(msg)
	switch action {
	case HomeActionStartCrawler:
		if m.Home.CrawlMode == CrawlMonitor {
			return m, crawlMonitorWithOptionsCmd(m.Client, m.Database, m.Home)
		}
		if m.Home.CrawlMode == CrawlFetchImages {
			return m, crawlFetchImagesCmd(m.Client, m.Database, m.Home.ConvertWebp)
		}
		if m.Home.CrawlMode == CrawlFetchThumbnails {
			return m, crawlFetchThumbnailsCmd(m.Client, m.Home.ThumbnailStartID, m.Home.ThumbnailEndID, m.Home.ConvertWebp)
		}
		return m, crawlPageWithOptionsCmd(m.Client, m.Database, 1, m.Home)
	case HomeActionStopCrawler:
		log.Printf("[Crawler] 爬虫已手动停止")
	}
	return m, nil
}

func (m Model) handleScheduleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.String() == "r" {
		m.Schedule.Loading = true
		m.Schedule.Error = ""
		return m, loadCourseScheduleCmd(m.Provider)
	}
	return m, nil
}

func (m Model) handleScoresKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		m.Scores.Loading = true
		m.Scores.Error = ""
		return m, loadScoresCmd(m.Provider)
	case "up":
		m.Scores.Move(-1, m.contentAreaHeightForSize(m.Width, m.Height))
	case "down":
		m.Scores.Move(1, m.contentAreaHeightForSize(m.Width, m.Height))
	case "pgup":
		m.Scores.Move(-maxInt(1, m.contentAreaHeightForSize(m.Width, m.Height)-16), m.contentAreaHeightForSize(m.Width, m.Height))
	case "pgdown":
		m.Scores.Move(maxInt(1, m.contentAreaHeightForSize(m.Width, m.Height)-16), m.contentAreaHeightForSize(m.Width, m.Height))
	}
	return m, nil
}

func (m Model) handlePostsKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if m.Posts.Searching {
		if m.Posts.SearchField.Value() != m.Posts.SearchInput {
			m.Posts.SearchField.SetValue(m.Posts.SearchInput)
		}
		if !m.Posts.SearchField.Focused() {
			_ = m.Posts.SearchField.Focus()
			switch msg.Code {
			case tea.KeyLeft:
				if pos := m.Posts.SearchField.Position(); pos > 0 {
					m.Posts.SearchField.SetCursor(pos - 1)
				}
				m.syncPostsPage()
				return m, nil
			case tea.KeyRight:
				if pos := m.Posts.SearchField.Position(); pos < len([]rune(m.Posts.SearchField.Value())) {
					m.Posts.SearchField.SetCursor(pos + 1)
				}
				m.syncPostsPage()
				return m, nil
			}
		}
		switch msg.Code {
		case tea.KeyEscape:
			return m.cancelSearchInput()
		case tea.KeyEnter:
			if m.Posts.SearchInput != "" {
				m.Posts.PostListLoading = true
				m.Posts.PostsMode = PostsModeSearchInput
				return m, searchPostsCmd(m.Provider, m.Posts.SearchInput, 0, m.Posts.PostPerPage, m.Posts.ActiveTagID)
			}
			return m, nil
		default:
			var cmd tea.Cmd
			m.Posts.SearchField, cmd = m.Posts.SearchField.Update(msg)
			m.Posts.SearchInput = m.Posts.SearchField.Value()
			m.syncPostsPage()
			return m, m.imageRefreshCmd(cmd)
		}
	}

	if m.Posts.ShowPostDetail {
		switch msg.String() {
		case "esc":
			m.Posts.ShowPostDetail = false
			m.Posts.CurrentPost = nil
			m.Posts.postBodyContent = ""
			m.Posts.resetComments()
			m.Posts.commentContent = ""
			m.Posts.PostBodyViewport.GotoTop()
			m.Posts.DetailFocus = DetailFocusComments
			if m.Posts.SearchActive {
				m.Posts.PostsMode = PostsModeSearchResults
			} else {
				m.Posts.PostsMode = PostsModeList
			}
			m.syncPostsPage()
			return m, m.imageRefreshCmd(nil)
		case "tab":
			if m.Posts.DetailFocus == DetailFocusPost {
				m.Posts.DetailFocus = DetailFocusComments
			} else {
				m.Posts.DetailFocus = DetailFocusPost
			}
		case "s":
			if m.Posts.CurrentPost != nil {
				nextSortAsc := !m.Posts.CommentSortAsc
				m.Posts.resetComments()
				m.Posts.CommentListLoading = true
				return m, loadCommentsCmd(m.Provider, m.Posts.CurrentPost.Pid, nextSortAsc, 0)
			}
		case "r":
			if m.Posts.CurrentPost != nil {
				m.Posts.CommentListLoading = true
				m.Posts.CommentListError = ""
				return m, loadPostDetailCmd(m.Provider, m.Posts.CurrentPost.Pid, m.Posts.CommentSortAsc)
			}
		case "o":
			return m.openCurrentImagePanel()
		case "p":
			if m.Posts.CurrentPost != nil {
				if !m.Posts.CanWrite {
					return m, m.setWriteUnavailableStatus()
				}
				return m, togglePraiseCmd(m.Provider, m.Posts.CurrentPost.Pid)
			}
		case "f":
			if m.Posts.CurrentPost != nil {
				if !m.Posts.CanWrite {
					return m, m.setWriteUnavailableStatus()
				}
				return m, toggleAttentionCmd(m.Provider, m.Posts.CurrentPost.Pid)
			}
		case "c":
			if m.Posts.CurrentPost != nil {
				if !m.Posts.CanWrite {
					return m, m.setWriteUnavailableStatus()
				}
				m.Composer.Configure(ComposerModeComment)
				m.Dialog = DialogComposer
				return m, nil
			}
		case "q":
			if m.Posts.CurrentPost != nil {
				if !m.Posts.CanWrite {
					return m, m.setWriteUnavailableStatus()
				}
				quoted := m.Posts.SelectedComment()
				if quoted == nil {
					return m, m.showToast("当前没有可引用的评论")
				}
				m.Composer.Configure(ComposerModeComment)
				m.Composer.SetQuoteTarget(quoted)
				m.Dialog = DialogComposer
				return m, nil
			}
		case "up":
			if m.Posts.DetailFocus == DetailFocusPost {
				m.Posts.PostBodyViewport.ScrollUp(1)
			} else {
				m.Posts.moveCommentSelection(-1)
			}
		case "down":
			if m.Posts.DetailFocus == DetailFocusPost {
				m.Posts.PostBodyViewport.ScrollDown(1)
			} else {
				m.Posts.moveCommentSelection(1)
				if m.Posts.CurrentPost != nil && m.Posts.shouldPrefetchCommentsMore() {
					m.Posts.CommentListLoading = true
					return m, loadCommentsCmd(m.Provider, m.Posts.CurrentPost.Pid, m.Posts.CommentSortAsc, m.Posts.CommentListCursor)
				}
			}
		case "pgup":
			if m.Posts.DetailFocus == DetailFocusPost {
				m.Posts.PostBodyViewport.PageUp()
			} else {
				m.Posts.commentPageMove(-1)
			}
		case "pgdown":
			if m.Posts.DetailFocus == DetailFocusPost {
				m.Posts.PostBodyViewport.PageDown()
			} else {
				m.Posts.commentPageMove(1)
				if m.Posts.CurrentPost != nil && m.Posts.shouldPrefetchCommentsMore() {
					m.Posts.CommentListLoading = true
					return m, loadCommentsCmd(m.Provider, m.Posts.CurrentPost.Pid, m.Posts.CommentSortAsc, m.Posts.CommentListCursor)
				}
			}
		}
		m.syncPostsPage()
		return m, m.imageRefreshCmd(nil)
	}

	switch msg.String() {
	case "esc":
		if m.Posts.SearchActive || m.Posts.ActiveTagID != 0 {
			return m.clearActiveFilters()
		}
	case "r":
		if !m.Posts.SearchActive {
			m.Posts.PostListLoading = true
			m.Posts.resetList()
			return m, loadPostsCmd(m.Provider, 0, m.Posts.PostPerPage, m.Posts.ActiveTagID)
		}
	case "o":
		return m.openCurrentImagePanel()
	case "/":
		m.Posts.Searching = true
		m.Posts.PostsMode = PostsModeSearchInput
		m.Posts.SearchInput = ""
		m.Posts.SearchField = newSearchInput()
		m.Posts.SearchField.SetValue("")
		_ = m.Posts.SearchField.Focus()
		return m, nil
	case "t":
		m.Dialog = DialogTags
		if len(m.TagsDialog.groups) == 0 && m.Provider.Mode() == SessionModeOnline {
			return m, loadTagsCmd(m.Provider)
		}
		return m, nil
	case "n":
		if !m.Posts.CanWrite {
			return m, m.setWriteUnavailableStatus()
		}
		m.Composer.Configure(ComposerModePost)
		m.Dialog = DialogComposer
		return m, nil
	case "p":
		if !m.Posts.CanWrite {
			return m, m.setWriteUnavailableStatus()
		}
		if post := m.Posts.SelectedPost(); post != nil {
			return m, togglePraiseCmd(m.Provider, post.Pid)
		}
	case "f":
		if !m.Posts.CanWrite {
			return m, m.setWriteUnavailableStatus()
		}
		if post := m.Posts.SelectedPost(); post != nil {
			return m, toggleAttentionCmd(m.Provider, post.Pid)
		}
	case "up":
		m.Posts.moveCursor(-1)
	case "down":
		m.Posts.moveCursor(1)
		if m.Posts.shouldPrefetchMore() {
			m.Posts.PostListLoading = true
			if m.Posts.SearchActive {
				return m, searchPostsCmd(m.Provider, m.Posts.SearchInput, m.Posts.PostListCursor, m.Posts.PostPerPage, m.Posts.ActiveTagID)
			}
			return m, loadPostsCmd(m.Provider, m.Posts.PostListCursor, m.Posts.PostPerPage, m.Posts.ActiveTagID)
		}
	case "enter":
		if post := m.Posts.SelectedPost(); post != nil {
			m.Posts.ShowPostDetail = true
			m.Posts.PostsMode = PostsModeDetail
			m.Posts.CurrentPost = post
			m.Posts.resetComments()
			m.Posts.CommentListLoading = true
			m.Posts.PostBodyViewport.GotoTop()
			m.Posts.DetailFocus = DetailFocusComments
			m.syncPostsPage()
			return m, m.imageRefreshCmd(loadPostDetailCmd(m.Provider, post.Pid, true))
		}
	case "pgup":
		m.Posts.pageMove(-1)
	case "pgdown":
		m.Posts.pageMove(1)
		if m.Posts.shouldPrefetchMore() && m.Posts.PostListHasMore && !m.Posts.PostListLoading {
			m.Posts.PostListLoading = true
			if m.Posts.SearchActive {
				return m, searchPostsCmd(m.Provider, m.Posts.SearchInput, m.Posts.PostListCursor, m.Posts.PostPerPage, m.Posts.ActiveTagID)
			}
			return m, loadPostsCmd(m.Provider, m.Posts.PostListCursor, m.Posts.PostPerPage, m.Posts.ActiveTagID)
		}
	}
	m.syncPostsPage()
	return m, m.imageRefreshCmd(nil)
}

func (m Model) cancelSearchInput() (Model, tea.Cmd) {
	m.Posts.Searching = false
	m.Posts.SearchInput = ""
	m.Posts.SearchField = newSearchInput()
	if m.Posts.SearchActive {
		m.Posts.PostsMode = PostsModeSearchResults
	} else {
		m.Posts.PostsMode = PostsModeList
	}
	m.syncPostsPage()
	return m, m.imageRefreshCmd(nil)
}

func (m Model) clearActiveFilters() (Model, tea.Cmd) {
	m.Posts.SearchActive = false
	m.Posts.Searching = false
	m.Posts.SearchInput = ""
	m.Posts.SearchField = newSearchInput()
	m.Posts.ActiveTagID = 0
	m.Posts.ActiveTag = ""
	m.Posts.PostsMode = PostsModeList
	m.Posts.PostListLoading = true
	m.Posts.resetList()
	m.syncPostsPage()
	return m, m.imageRefreshCmd(loadPostsCmd(m.Provider, 0, m.Posts.PostPerPage, 0))
}

func (m *Model) syncPostsPage() {
	m.Posts.SessionMode = m.Session.Mode
	m.Posts.CanWrite = m.Session.CanWriteOnline && m.Session.Mode == SessionModeOnline
	m.Posts.syncViewports(m.Width, m.contentAreaHeightForSize(m.Width, m.Height))
}

func (m Model) handleToolsDialogKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.Code == tea.KeyEscape &&
		(m.ToolsDialog.Section() != ToolsSectionConfig || m.ToolsDialog.Config.Mode() == ConfigEditorNormal) {
		m.Dialog = DialogNone
		return m, nil
	}
	if m.ToolsDialog.Section() == ToolsSectionConfig && msg.String() == "ctrl+s" {
		cfg, err := m.ToolsDialog.Config.ToConfig()
		if err != nil {
			m.ToolsDialog.Config.SetSaveResult(err)
			return m, nil
		}
		m.ToolsDialog.Config.SetSaving(true)
		return m, saveConfigCmd(cfg)
	}
	if m.ToolsDialog.Section() == ToolsSectionConfig && m.ToolsDialog.Config.Mode() == ConfigEditorInsert {
		m.ToolsDialog.Config.Update(msg)
		return m, nil
	}
	switch msg.String() {
	case "1":
		m.ToolsDialog.Switch(ToolsSectionConfig)
		return m, nil
	case "2":
		m.ToolsDialog.Switch(ToolsSectionLogs)
		m.ToolsDialog.Logs.SetLoading(true)
		return m, loadLogsCmd()
	case "3":
		m.ToolsDialog.Switch(ToolsSectionInteractive)
		m.ToolsDialog.Notifications.SetMessageType(models.NotificationTypeInteractive)
		m.ToolsDialog.Notifications.SetLoading(true)
		return m, loadNotificationsCmd(m.Client, m.ToolsDialog.Notifications.MessageType())
	case "4":
		m.ToolsDialog.Switch(ToolsSectionSystem)
		m.ToolsDialog.Notifications.SetMessageType(models.NotificationTypeSystem)
		m.ToolsDialog.Notifications.SetLoading(true)
		return m, loadNotificationsCmd(m.Client, m.ToolsDialog.Notifications.MessageType())
	}
	switch m.ToolsDialog.Section() {
	case ToolsSectionConfig:
		m.ToolsDialog.Config.Update(msg)
		return m, nil
	case ToolsSectionLogs:
		return m, m.ToolsDialog.Logs.Update(msg)
	}

	notifications := &m.ToolsDialog.Notifications
	switch msg.String() {
	case "r":
		notifications.SetLoading(true)
		return m, loadNotificationsCmd(m.Client, notifications.MessageType())
	case "enter":
		if !notifications.CanMarkSelectedRead() {
			return m, nil
		}
		selected := notifications.Selected()
		notifications.SetAction(true)
		return m, setNotificationReadCmd(m.Client, selected.ID, notifications.MessageType())
	case "a":
		notifications.SetAction(true)
		return m, setAllNotificationsReadCmd(m.Client, notifications.MessageType())
	}
	if notifications.Update(msg) {
		return m, loadNotificationsCmd(m.Client, notifications.MessageType())
	}
	return m, nil
}

func (m Model) handleImageDialogKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.Dialog = DialogNone
		m.ImageDialog.Clear()
	case "left":
		m.ImageDialog.Prev()
	case "right":
		m.ImageDialog.Next()
	}
	return m, nil
}

func (m Model) handleSessionDialogKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.Code == tea.KeyEscape {
		m.Dialog = DialogNone
		return m, nil
	}
	if msg.Code == tea.KeyEnter {
		switch m.SessionDialog.SelectedOption() {
		case "打开配置":
			m.Dialog = DialogTools
			m.ToolsDialog.Switch(ToolsSectionConfig)
			m.ToolsDialog.Config = NewConfigDialog(m.Config)
			return m, loadConfigCmd()
		case "重新登录":
			return m, refreshSessionCmd(m.Client, m.Config)
		case "进入离线模式", "确定":
			offlineCmd := m.forceOfflineMode(m.Session.Message)
			m.Dialog = DialogNone
			m.Posts.PostListLoading = true
			return m, tea.Batch(offlineCmd, loadPostsCmd(m.Provider, 0, m.Posts.PostPerPage, 0))
		}
	}
	m.SessionDialog.Update(msg)
	return m, nil
}

func (m Model) openCurrentImagePanel() (Model, tea.Cmd) {
	title, items := m.currentImageSelection()
	if len(items) == 0 {
		return m, m.showToast("当前选中内容没有可显示的图片")
	}
	m.ImageDialog.Open(title, items)
	m.Dialog = DialogImage
	return m, nil
}

func (m Model) currentImageSelection() (string, []resolvedMedia) {
	if m.Page != PagePosts {
		return "", nil
	}
	if m.Posts.ShowPostDetail {
		if comment := m.Posts.SelectedComment(); comment != nil && strings.TrimSpace(comment.MediaIds) != "" {
			title := fmt.Sprintf("评论图片 %d/%d", m.Posts.SelectedCommentIdx+1, len(m.Posts.CommentList))
			items := resolveMediaPathsWithClient(m.Client, comment.MediaIds, true)
			if len(items) > 0 {
				return title, items
			}
		}
		if m.Posts.CurrentPost != nil {
			title := fmt.Sprintf("帖子 #%d", m.Posts.CurrentPost.Pid)
			items := resolvePostMediaPathsWithClient(
				m.Client,
				m.Posts.CurrentPost.Pid,
				m.Posts.CurrentPost.Type,
				m.Posts.CurrentPost.MediaIds,
				true,
			)
			return title, items
		}
		return "", nil
	}

	post := m.Posts.SelectedPost()
	if post == nil {
		return "", nil
	}
	title := fmt.Sprintf("帖子 #%d", post.Pid)
	return title, resolvePostMediaPathsWithClient(m.Client, post.Pid, post.Type, post.MediaIds, true)
}

func (m Model) handleAuthChallengeKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.Code == tea.KeyEscape {
		reason := m.Session.ChallengeMessage
		if reason == "" {
			reason = m.Session.Message
		}
		offlineCmd := m.forceOfflineMode(reason)
		m.Dialog = DialogNone
		m.Posts.PostListLoading = true
		return m, tea.Batch(offlineCmd, loadPostsCmd(m.Provider, 0, m.Posts.PostPerPage, 0))
	}
	if msg.String() == "ctrl+r" && m.AuthDialog.Kind() == AuthChallengeTypeSMS {
		m.AuthDialog.SetSubmitting(true)
		m.AuthDialog.SetError(nil)
		m.AuthDialog.SetStatus("")
		m.AuthDialog.MarkSMSSent()
		return m, sendSMSChallengeCmd(m.Client)
	}
	if msg.Code == tea.KeyEnter {
		if m.AuthDialog.Kind() == AuthChallengeTypeSMS && m.AuthDialog.IsSendFocused() {
			m.AuthDialog.SetSubmitting(true)
			m.AuthDialog.SetError(nil)
			m.AuthDialog.SetStatus("")
			m.AuthDialog.MarkSMSSent()
			return m, sendSMSChallengeCmd(m.Client)
		}
		code := m.AuthDialog.Value()
		if code == "" {
			if m.AuthDialog.Kind() == AuthChallengeTypeUsername {
				m.AuthDialog.SetError(errors.New("用户名不能为空"))
			} else if m.AuthDialog.Kind() == AuthChallengeTypePassword {
				m.AuthDialog.SetError(errors.New("密码不能为空"))
			} else {
				m.AuthDialog.SetError(errors.New("验证码不能为空"))
			}
			return m, nil
		}
		m.AuthDialog.SetSubmitting(true)
		m.AuthDialog.SetError(nil)
		m.AuthDialog.SetStatus("")
		if m.AuthDialog.Kind() == AuthChallengeTypeUsername {
			if m.Config == nil {
				defaultCfg := config.DefaultConfig()
				m.Config = &defaultCfg
			}
			m.Config.Username = strings.TrimSpace(code)
			return m, submitUsernameChallengeCmd(m.Client, m.Config, code)
		}
		if m.AuthDialog.Kind() == AuthChallengeTypePassword {
			return m, submitPasswordLoginCmd(m.Client, m.Config, code)
		}
		return m, submitAuthChallengeCmd(m.Client, m.AuthDialog.Kind(), code)
	}
	cmd := m.AuthDialog.Update(msg)
	return m, cmd
}

func (m Model) handleComposerKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.Code == tea.KeyEscape {
		m.Dialog = DialogNone
		return m, nil
	}
	if msg.String() == "ctrl+s" {
		text := m.Composer.Value()
		if text == "" {
			m.Composer.SetError(errors.New("内容不能为空"))
			return m, nil
		}
		imagePaths := m.Composer.ImagePaths()
		if m.Composer.Mode() == ComposerModeComment && m.Posts.CurrentPost != nil {
			return m, createCommentCmd(m.Provider, m.Posts.CurrentPost.Pid, text, m.Composer.QuoteTarget(), imagePaths)
		}
		return m, createPostCmd(m.Provider, text, imagePaths)
	}
	cmd := m.Composer.Update(msg)
	return m, cmd
}

func (m Model) handleTagsDialogKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if msg.Code == tea.KeyEscape {
		m.Dialog = DialogNone
		return m, nil
	}
	switch msg.String() {
	case "left", "h", "backspace":
		if m.TagsDialog.Back() {
			return m, nil
		}
	case "c":
		m.Posts.ActiveTagID = 0
		m.Posts.ActiveTag = ""
		m.Dialog = DialogNone
		m.Posts.PostListLoading = true
		return m, loadPostsCmd(m.Provider, 0, m.Posts.PostPerPage, 0)
	case "enter":
		if !m.TagsDialog.Enter() {
			return m, nil
		}
		tag := m.TagsDialog.SelectedTag()
		if tag != nil {
			m.Posts.SearchActive = false
			m.Posts.Searching = false
			m.Posts.SearchInput = ""
			m.Posts.ActiveTagID = tag.ID
			m.Posts.ActiveTag = tag.Label
			if m.Posts.ActiveTag == "" {
				m.Posts.ActiveTag = tag.Name
			}
			m.Dialog = DialogNone
			m.Posts.PostListLoading = true
			m.Posts.resetList()
			return m, loadPostsCmd(m.Provider, 0, m.Posts.PostPerPage, m.Posts.ActiveTagID)
		}
	}
	m.TagsDialog.Update(msg)
	return m, nil
}

func crawlPageCmd(c *client.Client, database *db.Database, page int) tea.Cmd {
	options := NewHomePageModel()
	return crawlPageWithOptionsCmd(c, database, page, options)
}

func crawlPageWithOptionsCmd(c *client.Client, database *db.Database, page int, options HomePageModel) tea.Cmd {
	return func() tea.Msg {
		startTime := time.Now()
		result, err := crawler.FetchAndSave(c, database, page, options.SaveJSON, options.PostsPerRequest, options.CommentsPerPost, options.FetchImages, options.ConvertWebp)
		duration := time.Since(startTime)
		if err != nil {
			return CrawlMsg{Error: err, Page: page}
		}
		if options.SaveJSON {
			if err := crawler.SaveRawResponsesToFile(); err != nil {
				return CrawlMsg{Error: err, Page: page}
			}
		}
		return CrawlMsg{
			Page:     page,
			Duration: duration,
			Summary:  fmt.Sprintf("第%d页：帖子 %d，评论 %d", page, result.PostCount, result.CommentCount),
		}
	}
}

func crawlMonitorCmd(c *client.Client, database *db.Database, monitorPages int) tea.Cmd {
	options := NewHomePageModel()
	options.MonitorPages = monitorPages
	return crawlMonitorWithOptionsCmd(c, database, options)
}

func crawlMonitorWithOptionsCmd(c *client.Client, database *db.Database, options HomePageModel) tea.Cmd {
	return func() tea.Msg {
		startTime := time.Now()
		totalPosts := 0
		totalComments := 0
		for page := 1; page <= options.MonitorPages; page++ {
			result, err := crawler.FetchAndSave(c, database, page, options.SaveJSON, options.PostsPerRequest, options.CommentsPerPost, options.FetchImages, options.ConvertWebp)
			if err != nil {
				log.Printf("[Crawler] 监控模式第 %d 页抓取失败: %v", page, err)
				continue
			}
			totalPosts += result.PostCount
			totalComments += result.CommentCount
		}
		if options.SaveJSON {
			if err := crawler.SaveRawResponsesToFile(); err != nil {
				return CrawlMsg{Error: err, Page: options.MonitorPages}
			}
		}
		return CrawlMsg{
			Page:     options.MonitorPages,
			Duration: time.Since(startTime),
			Summary:  fmt.Sprintf("监控前%d页：帖子 %d，评论 %d", options.MonitorPages, totalPosts, totalComments),
		}
	}
}

func crawlFetchImagesCmd(c *client.Client, database *db.Database, convertWebp bool) tea.Cmd {
	return func() tea.Msg {
		startTime := time.Now()
		crawler.FetchImagesFromDB(c, database, convertWebp)
		return CrawlMsg{Duration: time.Since(startTime), Summary: "数据库图片补齐完成"}
	}
}

func crawlFetchThumbnailsCmd(c *client.Client, startID, endID int, convertWebp bool) tea.Cmd {
	return func() tea.Msg {
		startTime := time.Now()
		downloaded, skipped, err := crawler.FetchThumbnailsByIDRange(c, startID, endID, convertWebp)
		if err != nil {
			return CrawlMsg{Error: err}
		}
		return CrawlMsg{
			Duration: time.Since(startTime),
			Summary:  fmt.Sprintf("缩略图完成：downloaded=%d skipped=%d range=%d-%d", downloaded, skipped, startID, endID),
		}
	}
}

func loadPostsCmd(provider PostsProvider, cursor, limit, label int) tea.Cmd {
	return func() tea.Msg {
		posts, nextCursor, hasMore, err := provider.ListPosts(cursor, limit, label, "")
		if err != nil {
			return LoadPostsMsg{Error: err}
		}
		return LoadPostsMsg{Posts: posts, RequestCursor: cursor, NextCursor: nextCursor, HasMore: hasMore}
	}
}

func loadCommentsCmd(provider PostsProvider, pid int32, sortAsc bool, cursor ...int32) tea.Cmd {
	return func() tea.Msg {
		begin := int32(0)
		if len(cursor) > 0 {
			begin = cursor[0]
		}
		comments, next, hasMore, err := provider.ListComments(pid, sortAsc, begin)
		if err != nil {
			return LoadCommentsMsg{Error: err}
		}
		return LoadCommentsMsg{Comments: comments, RequestCursor: begin, NextCursor: next, HasMore: hasMore, SortAsc: sortAsc}
	}
}

func loadPostDetailCmd(provider PostsProvider, pid int32, sortAsc bool) tea.Cmd {
	return func() tea.Msg {
		post, comments, next, hasMore, err := provider.GetPostDetail(pid, sortAsc)
		if err != nil {
			return LoadPostDetailMsg{Error: err}
		}
		return LoadPostDetailMsg{Post: post, Comments: comments, NextCursor: next, HasMore: hasMore, SortAsc: sortAsc}
	}
}

func searchPostsCmd(provider PostsProvider, keyword string, cursor, limit, label int) tea.Cmd {
	return func() tea.Msg {
		posts, nextCursor, hasMore, err := provider.SearchPosts(keyword, cursor, limit, label)
		if err != nil {
			return SearchPostsMsg{Error: err}
		}
		return SearchPostsMsg{Posts: posts, RequestCursor: cursor, NextCursor: nextCursor, HasMore: hasMore}
	}
}

func loadTagsCmd(provider PostsProvider) tea.Cmd {
	return func() tea.Msg {
		tags, err := provider.ListTags()
		return LoadTagsMsg{Tags: tags, Error: err}
	}
}

func loadCourseScheduleCmd(provider PostsProvider) tea.Cmd {
	return func() tea.Msg {
		rows, err := provider.GetCourseTable()
		return LoadCourseScheduleMsg{Rows: rows, Error: err}
	}
}

func loadScoresCmd(provider PostsProvider) tea.Cmd {
	return func() tea.Msg {
		summary, err := provider.GetCourseScores()
		return LoadScoresMsg{Summary: summary, Error: err}
	}
}

func togglePraiseCmd(provider PostsProvider, pid int32) tea.Cmd {
	return func() tea.Msg {
		if err := provider.TogglePraise(pid); err != nil {
			return ActionResultMsg{Kind: "praise", Error: err}
		}
		post, err := provider.RefreshPost(pid)
		if err != nil {
			return ActionResultMsg{Kind: "praise", Error: err}
		}
		return ActionResultMsg{Kind: "praise", Message: "点赞状态已刷新", Post: post}
	}
}

func toggleAttentionCmd(provider PostsProvider, pid int32) tea.Cmd {
	return func() tea.Msg {
		if err := provider.ToggleAttention(pid); err != nil {
			return ActionResultMsg{Kind: "attention", Error: err}
		}
		post, err := provider.RefreshPost(pid)
		if err != nil {
			return ActionResultMsg{Kind: "attention", Error: err}
		}
		return ActionResultMsg{Kind: "attention", Message: "关注状态已刷新", Post: post}
	}
}

func createCommentCmd(provider PostsProvider, pid int32, text string, quote *models.Comment, imagePaths []string) tea.Cmd {
	return func() tea.Msg {
		var quoteID *int32
		if quote != nil {
			quoteID = &quote.Cid
		}
		err := provider.CreateComment(pid, text, quoteID, imagePaths)
		if err != nil {
			return ActionResultMsg{Kind: "comment", Error: err}
		}
		return ActionResultMsg{Kind: "comment", Message: "评论发布成功"}
	}
}

func createPostCmd(provider PostsProvider, text string, imagePaths []string) tea.Cmd {
	return func() tea.Msg {
		err := provider.CreatePost(text, imagePaths)
		if err != nil {
			return ActionResultMsg{Kind: "post", Error: err}
		}
		return ActionResultMsg{Kind: "post", Message: "帖子发布成功"}
	}
}

func nextPostCursor(posts []models.Post) int {
	if len(posts) == 0 {
		return 0
	}
	return int(posts[len(posts)-1].Pid)
}

func nextCommentCursor(comments []models.Comment) int32 {
	if len(comments) == 0 {
		return 0
	}
	return comments[len(comments)-1].Cid
}

func loadLogsCmd() tea.Cmd {
	return func() tea.Msg {
		if err := config.EnsureRuntimeFiles(); err != nil {
			return LoadLogsMsg{Error: err}
		}
		logPath, err := config.LogPath()
		if err != nil {
			return LoadLogsMsg{Error: err}
		}
		file, err := os.Open(logPath)
		if err != nil {
			return LoadLogsMsg{Error: err}
		}
		defer file.Close()
		var lines []string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if len(lines) > 500 {
			lines = lines[len(lines)-500:]
		}
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
		return LoadLogsMsg{Lines: lines}
	}
}

func loadNotificationsCmd(c *client.Client, messageType models.NotificationType) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return LoadNotificationsMsg{MessageType: messageType, Error: errors.New("客户端未初始化")}
		}
		items, total, err := c.ListNotificationsV3(messageType, 1, 50)
		return LoadNotificationsMsg{MessageType: messageType, Items: items, Total: total, Error: err}
	}
}

func loadDashboardNotificationsCmd(c *client.Client) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return LoadDashboardNotificationsMsg{Error: errors.New("客户端未初始化")}
		}
		interactive, _, intErr := c.ListNotificationsV3(models.NotificationTypeInteractive, 1, 20)
		system, _, sysErr := c.ListNotificationsV3(models.NotificationTypeSystem, 1, 20)
		if intErr != nil && sysErr != nil {
			return LoadDashboardNotificationsMsg{Error: intErr}
		}
		items := make([]models.Notification, 0, len(interactive)+len(system))
		for _, item := range append(interactive, system...) {
			if !item.Read {
				items = append(items, item)
			}
		}
		sort.SliceStable(items, func(i, j int) bool {
			left := items[i].Timestamp
			right := items[j].Timestamp
			if left == 0 && items[i].CreatedAt != "" {
				if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", items[i].CreatedAt, shanghaiLocation); err == nil {
					left = parsed.Unix()
				}
			}
			if right == 0 && items[j].CreatedAt != "" {
				if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", items[j].CreatedAt, shanghaiLocation); err == nil {
					right = parsed.Unix()
				}
			}
			return left > right
		})
		return LoadDashboardNotificationsMsg{Items: items}
	}
}

func setNotificationReadCmd(c *client.Client, id int, messageType models.NotificationType) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return NotificationActionMsg{MessageType: messageType, ID: id, Error: errors.New("客户端未初始化")}
		}
		err := c.SetNotificationReadV3(id)
		return NotificationActionMsg{MessageType: messageType, ID: id, Error: err}
	}
}

func setAllNotificationsReadCmd(c *client.Client, messageType models.NotificationType) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return NotificationActionMsg{MessageType: messageType, All: true, Error: errors.New("客户端未初始化")}
		}
		err := c.SetAllNotificationsReadV3(messageType)
		return NotificationActionMsg{MessageType: messageType, All: true, Error: err}
	}
}

func loadConfigCmd() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.LoadConfig()
		if err != nil {
			return LoadConfigMsg{Error: err}
		}
		return LoadConfigMsg{Config: cfg}
	}
}

func saveConfigCmd(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		if err := config.EnsureRuntimeFiles(); err != nil {
			return SaveConfigMsg{Error: err}
		}
		data, err := json.MarshalIndent(cfg, "", "    ")
		if err != nil {
			return SaveConfigMsg{Error: err}
		}
		configPath, err := config.ConfigPath()
		if err != nil {
			return SaveConfigMsg{Error: err}
		}
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return SaveConfigMsg{Error: err}
		}
		return SaveConfigMsg{Config: cfg}
	}
}

func InitClientForTUI() (*client.Client, *config.Config, SessionState, error) {
	cfg, cfgErr := config.LoadConfig()
	deviceUUID := ""
	if cfgErr == nil && cfg != nil {
		deviceUUID = cfg.DeviceUUID
	}
	c, err := client.NewClient(deviceUUID)
	if err != nil {
		return nil, nil, SessionState{}, err
	}
	state := attemptBootstrapSession(c, cfg)
	if cfg == nil && cfgErr == nil {
		cfg, _ = config.LoadConfig()
	}
	return c, cfg, state, nil
}

func refreshSessionCmd(c *client.Client, cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		return SessionRefreshMsg{State: attemptBootstrapSession(c, cfg)}
	}
}

func attemptBootstrapSession(c *client.Client, cfg *config.Config) SessionState {
	state := toTUISessionState(c.BootstrapSession(cfg))
	if !state.CanReadOnline && state.Challenge == AuthChallengeTypeNone && (cfg == nil || !cfg.HasAnyPasswordLoginInput()) {
		state.FailureReason = SessionFailureReasonLogin
		state.NeedsConfig = true
		if state.Message == "" || state.Message == "登录态不可用" {
			state.Message = "未检测到可用登录态，也未配置账号密码。请先打开配置填写账号密码。"
		}
	}
	return state
}

func submitAuthChallengeCmd(c *client.Client, kind AuthChallengeType, code string) tea.Cmd {
	return func() tea.Msg {
		result := c.ContinueAuthChallenge(toClientAuthChallenge(kind), code)
		return AuthChallengeResultMsg{State: toTUISessionState(result)}
	}
}

func submitPasswordLoginCmd(c *client.Client, cfg *config.Config, password string) tea.Cmd {
	return func() tea.Msg {
		result := c.BootstrapSessionWithPassword(cfg, password)
		return AuthChallengeResultMsg{State: toTUISessionState(result)}
	}
}

func submitUsernameChallengeCmd(c *client.Client, cfg *config.Config, username string) tea.Cmd {
	return func() tea.Msg {
		trimmed := strings.TrimSpace(username)
		if trimmed == "" {
			return AuthChallengeResultMsg{State: SessionState{
				Mode:             SessionModeOffline,
				Challenge:        AuthChallengeTypeUsername,
				ChallengeMessage: "用户名不能为空",
				Message:          "用户名不能为空",
			}}
		}
		if cfg == nil {
			defaultCfg := config.DefaultConfig()
			cfg = &defaultCfg
		}
		cfg.Username = trimmed
		if strings.TrimSpace(cfg.Password) == "" {
			return AuthChallengeResultMsg{State: SessionState{
				Mode:             SessionModeOffline,
				Challenge:        AuthChallengeTypePassword,
				ChallengeMessage: "请输入用户密码",
				Message:          "请输入用户密码",
			}}
		}
		result := c.BootstrapSessionWithPassword(cfg, cfg.Password)
		return AuthChallengeResultMsg{State: toTUISessionState(result)}
	}
}

func sendSMSChallengeCmd(c *client.Client) tea.Cmd {
	return func() tea.Msg {
		if err := c.SendSMSCode(); err != nil {
			return AuthSMSSentMsg{Error: err}
		}
		return AuthSMSSentMsg{Message: "验证码已发送，请查收短信"}
	}
}

func toTUISessionState(result client.AuthBootstrapResult) SessionState {
	status := result.Status
	state := SessionState{
		HasSession:     status.HasSession,
		CanReadOnline:  status.CanReadOnline,
		CanWriteOnline: status.CanWriteOnline,
		Message:        status.Message,
		Challenge:      toTUIAuthChallenge(result.Challenge),
		ChallengeMessage: func() string {
			if result.ChallengeReason != "" {
				return result.ChallengeReason
			}
			return status.Message
		}(),
	}
	if status.CanReadOnline {
		state.Mode = SessionModeOnline
		state.LastFallbackReason = ""
		state.Challenge = AuthChallengeTypeNone
		state.ChallengeMessage = ""
		return state
	}
	state.Mode = SessionModeOffline
	state.LastFallbackReason = status.Message
	state.FailureReason = failureReasonFromClient(status.FailureKind)
	if state.Challenge != AuthChallengeTypeNone {
		state.FailureReason = SessionFailureReasonNone
	}
	return state
}

func toTUIAuthChallenge(kind client.AuthChallengeKind) AuthChallengeType {
	switch kind {
	case client.AuthChallengeUsername:
		return AuthChallengeTypeUsername
	case client.AuthChallengeSMS:
		return AuthChallengeTypeSMS
	case client.AuthChallengeOTP:
		return AuthChallengeTypeOTP
	case client.AuthChallengePassword:
		return AuthChallengeTypePassword
	default:
		return AuthChallengeTypeNone
	}
}

func toClientAuthChallenge(kind AuthChallengeType) client.AuthChallengeKind {
	switch kind {
	case AuthChallengeTypeUsername:
		return client.AuthChallengeUsername
	case AuthChallengeTypePassword:
		return client.AuthChallengePassword
	case AuthChallengeTypeSMS:
		return client.AuthChallengeSMS
	case AuthChallengeTypeOTP:
		return client.AuthChallengeOTP
	default:
		return client.AuthChallengeNone
	}
}

func failureReasonFromClient(kind client.SessionFailureKind) SessionFailureReason {
	switch kind {
	case client.SessionFailureNetwork:
		return SessionFailureReasonNetwork
	case client.SessionFailureLogin:
		return SessionFailureReasonLogin
	default:
		return SessionFailureReasonNone
	}
}

func (m *Model) handleOnlineReadFailure(err error) {
	if m.Provider == nil || m.Provider.Mode() != SessionModeOnline {
		return
	}
	state := SessionState{
		Mode:          SessionModeOffline,
		FailureReason: failureReasonFromClient(client.ClassifySessionError(err)),
		Message:       err.Error(),
	}
	m.Session = state
	m.SessionDialog.ApplyState(state)
	m.Dialog = DialogSessionPrompt
}

func (m *Model) applySessionState(state SessionState) tea.Cmd {
	m.Session = state
	var cmd tea.Cmd
	if state.CanReadOnline {
		m.Provider = NewOnlinePostsProvider(m.Client)
		m.Session.Mode = SessionModeOnline
		m.Home.LoggedIn = true
	} else {
		m.Provider = NewOfflinePostsProvider(m.Database)
		m.Session.Mode = SessionModeOffline
		m.Posts.ActiveTagID = 0
		m.Posts.ActiveTag = ""
		if state.Message != "" {
			cmd = m.showToast("离线模式：" + state.Message)
		}
		m.Home.LoggedIn = false
	}
	m.SessionDialog.ApplyState(state)
	m.AuthDialog.ApplyState(state)
	m.syncPostsPage()
	return cmd
}

func (m *Model) forceOfflineMode(reason string) tea.Cmd {
	m.Session.Mode = SessionModeOffline
	m.Session.CanReadOnline = false
	m.Session.CanWriteOnline = false
	m.Session.LastFallbackReason = reason
	m.Provider = NewOfflinePostsProvider(m.Database)
	m.Posts.ActiveTagID = 0
	m.Posts.ActiveTag = ""
	m.Home.LoggedIn = false
	var cmd tea.Cmd
	if reason != "" {
		cmd = m.showToast("离线模式：" + reason)
	}
	m.syncPostsPage()
	return cmd
}

func (m *Model) setWriteUnavailableStatus() tea.Cmd {
	if m.Session.Mode == SessionModeOnline {
		return m.showToast("当前在线会话不可写，请先重新登录或稍后再试")
	}
	return m.showToast("当前为离线模式，写操作不可用")
}
