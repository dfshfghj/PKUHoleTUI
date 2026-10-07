package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/pflag"
)

type parsedCrawlerCommand struct {
	subcommand     string
	startPage      int
	pages          int
	loop           bool
	loopDelay      int
	pageDelay      int
	resume         bool
	saveRawJSON    bool
	postBatchSize  int
	commentLimit   int
	fetchImages    bool
	convertWebp    bool
	thumbnailStart int
	thumbnailEnd   int
}

func splitCommandLine(input string) ([]string, error) {
	var (
		args     []string
		current  strings.Builder
		inSingle bool
		inDouble bool
		escaping bool
	)

	flush := func() {
		if current.Len() > 0 {
			args = append(args, current.String())
			current.Reset()
		}
	}

	for _, r := range input {
		switch {
		case escaping:
			current.WriteRune(r)
			escaping = false
		case r == '\\' && !inSingle:
			escaping = true
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case (r == ' ' || r == '\t') && !inSingle && !inDouble:
			flush()
		default:
			current.WriteRune(r)
		}
	}

	if escaping {
		current.WriteRune('\\')
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quote")
	}
	flush()
	return args, nil
}

func parseCrawlerCommand(args []string) (parsedCrawlerCommand, error) {
	cfg := parsedCrawlerCommand{
		startPage:     1,
		pages:         0,
		loopDelay:     60,
		pageDelay:     1,
		postBatchSize: 200,
		commentLimit:  200,
		convertWebp:   true,
	}

	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cfg.subcommand = args[0]
		args = args[1:]
	}

	switch cfg.subcommand {
	case "", "fetch-images", "fetch-thumbnails":
	default:
		return cfg, fmt.Errorf("unknown crawler subcommand: %s", cfg.subcommand)
	}

	fs := pflag.NewFlagSet("crawler", pflag.ContinueOnError)
	fs.ParseErrorsWhitelist.UnknownFlags = false
	fs.SetInterspersed(true)

	switch cfg.subcommand {
	case "":
		fs.IntVar(&cfg.startPage, "start-page", 1, "")
		fs.IntVar(&cfg.pages, "pages", 0, "")
		fs.BoolVar(&cfg.loop, "loop", false, "")
		fs.IntVar(&cfg.loopDelay, "loop-delay", 60, "")
		fs.IntVar(&cfg.pageDelay, "page-delay", 1, "")
		fs.BoolVar(&cfg.resume, "resume", false, "")
		fs.BoolVar(&cfg.saveRawJSON, "save-raw-json", false, "")
		fs.IntVar(&cfg.postBatchSize, "post-batch-size", 200, "")
		fs.IntVar(&cfg.commentLimit, "comment-limit", 200, "")
		fs.BoolVar(&cfg.fetchImages, "fetch-images", false, "")
		fs.BoolVar(&cfg.convertWebp, "convert-webp", true, "")
	case "fetch-images":
		fs.BoolVar(&cfg.convertWebp, "convert-webp", true, "")
	case "fetch-thumbnails":
		cfg.convertWebp = false
		fs.IntVar(&cfg.thumbnailStart, "start-id", 0, "")
		fs.IntVar(&cfg.thumbnailEnd, "end-id", 0, "")
		fs.BoolVar(&cfg.convertWebp, "convert-webp", false, "")
	}

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	if cfg.subcommand == "fetch-thumbnails" {
		if cfg.thumbnailStart <= 0 || cfg.thumbnailEnd <= 0 {
			return cfg, fmt.Errorf("fetch-thumbnails requires --start-id and --end-id")
		}
	}

	return cfg, nil
}

func (m Model) executeCrawlerCommand(args []string) (Model, tea.Cmd) {
	cfg, err := parseCrawlerCommand(args)
	if err != nil {
		return m, m.showToast("crawler 参数错误: " + err.Error())
	}
	params := defaultCrawlerParams()
	params.ConvertWebp = cfg.convertWebp
	name := "命令行任务"
	switch cfg.subcommand {
	case "fetch-images":
		params.Mode = CrawlFetchImages
		name = "补齐图片"
	case "fetch-thumbnails":
		params.Mode = CrawlFetchThumbnails
		params.ThumbnailStartID = cfg.thumbnailStart
		params.ThumbnailEndID = cfg.thumbnailEnd
		name = fmt.Sprintf("缩略图 %d-%d", cfg.thumbnailStart, cfg.thumbnailEnd)
	default:
		params.Mode = CrawlSequential
		if cfg.loop && cfg.pages > 0 && cfg.startPage == 1 {
			params.Mode = CrawlMonitor
			params.MonitorPages = cfg.pages
			name = fmt.Sprintf("监控前%d页", cfg.pages)
		}
		params.StartPage = cfg.startPage
		params.Pages = cfg.pages
		params.Loop = cfg.loop
		params.LoopDelaySeconds = cfg.loopDelay
		params.PageDelaySeconds = cfg.pageDelay
		params.FetchImages = cfg.fetchImages
		params.SaveRawJSON = cfg.saveRawJSON
		params.PostBatchSize = cfg.postBatchSize
		params.CommentLimit = cfg.commentLimit
	}

	m.Home.Jobs = append(m.Home.Jobs, newCrawlerJob(name, params))
	m.Home.SelectedJob = len(m.Home.Jobs) - 1
	m.Home.Focus = HomeFocusJobs
	if m.Page != PageHome {
		next, pageCmd := m.switchPage(PageHome)
		m = next
		started, startCmd := m.startBackgroundJob(m.Home.SelectedJob)
		return started, tea.Batch(pageCmd, startCmd)
	}
	return m.startBackgroundJob(m.Home.SelectedJob)
}
