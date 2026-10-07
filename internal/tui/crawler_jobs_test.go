package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The generated argv is the contract with the crawler CLI, so pin it down.
func TestCrawlerJobArgsMatchCLIFlags(t *testing.T) {
	tests := []struct {
		name string
		job  CrawlerJob
		want []string
	}{
		{
			name: "sequential one-shot",
			job: CrawlerJob{Params: CrawlerParams{
				Mode: CrawlSequential, StartPage: 5, Pages: 10, PageDelaySeconds: 2,
				PostBatchSize: 80, CommentLimit: 90, ConvertWebp: false,
			}},
			want: []string{"crawler", "--start-page", "5", "--pages", "10", "--page-delay", "2",
				"--post-batch-size", "80", "--comment-limit", "90", "--convert-webp=false"},
		},
		{
			name: "sequential looped with output flags",
			job: CrawlerJob{Params: CrawlerParams{
				Mode: CrawlSequential, StartPage: 1, Pages: 0, Loop: true, LoopDelaySeconds: 7,
				PageDelaySeconds: 1, PostBatchSize: 200, CommentLimit: 200,
				FetchImages: true, SaveRawJSON: true, ConvertWebp: true,
			}},
			want: []string{"crawler", "--start-page", "1", "--pages", "0", "--page-delay", "1",
				"--loop", "--loop-delay", "7", "--post-batch-size", "200", "--comment-limit", "200",
				"--fetch-images", "--save-raw-json", "--convert-webp=true"},
		},
		{
			name: "monitor forces newest pages and loop but keeps delays and output flags",
			job: CrawlerJob{Params: CrawlerParams{
				Mode: CrawlMonitor, StartPage: 99, Pages: 42, MonitorPages: 3, Loop: false,
				LoopDelaySeconds: 60, PageDelaySeconds: 5, PostBatchSize: 200, CommentLimit: 200,
				FetchImages: true, SaveRawJSON: true, ConvertWebp: true,
			}},
			want: []string{"crawler", "--start-page", "1", "--pages", "3", "--page-delay", "5",
				"--loop", "--loop-delay", "60", "--post-batch-size", "200", "--comment-limit", "200",
				"--fetch-images", "--save-raw-json", "--convert-webp=true"},
		},
		{
			name: "fetch images subcommand",
			job:  CrawlerJob{Params: CrawlerParams{Mode: CrawlFetchImages, ConvertWebp: true}},
			want: []string{"crawler", "fetch-images", "--convert-webp=true"},
		},
		{
			name: "thumbnails subcommand",
			job: CrawlerJob{Params: CrawlerParams{
				Mode: CrawlFetchThumbnails, ThumbnailStartID: 10, ThumbnailEndID: 20, ConvertWebp: false,
			}},
			want: []string{"crawler", "fetch-thumbnails", "--start-id", "10", "--end-id", "20",
				"--convert-webp=false"},
		},
		{
			name: "thumbnails end before start clamps",
			job: CrawlerJob{Params: CrawlerParams{
				Mode: CrawlFetchThumbnails, ThumbnailStartID: 50, ThumbnailEndID: 20, ConvertWebp: true,
			}},
			want: []string{"crawler", "fetch-thumbnails", "--start-id", "50", "--end-id", "50",
				"--convert-webp=true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jobArgs(tt.job)
			if strings.Join(got, " ") != strings.Join(tt.want, " ") {
				t.Fatalf("jobArgs =\n  %v\nwant\n  %v", got, tt.want)
			}
		})
	}
}

func TestCrawlerJobRegistryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	useTempWorkingDir(t, dir)

	jobs := []CrawlerJob{
		newCrawlerJob("全量循环", CrawlerParams{Mode: CrawlSequential, StartPage: 3, Loop: true}),
		newCrawlerJob("补图", CrawlerParams{Mode: CrawlFetchImages, ConvertWebp: true}),
	}
	for i := range jobs {
		jobs[i].PID = 4242 + i
		jobs[i].StartedAt = time.Unix(1700000000, 0)
	}
	if err := saveCrawlerJobs(jobs); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := loadCrawlerJobs()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded %d jobs, want 2", len(loaded))
	}
	if loaded[0].Name != "全量循环" || !loaded[0].Params.Loop || loaded[0].Params.StartPage != 3 {
		t.Fatalf("first job round-trip mismatch: %+v", loaded[0])
	}
	if loaded[1].Params.Mode != CrawlFetchImages {
		t.Fatalf("second job mode = %v, want fetch-images", loaded[1].Params.Mode)
	}
	if _, err := os.Stat(filepath.Join(dir, "data", "crawler-jobs.json")); err != nil {
		t.Fatalf("registry file missing: %v", err)
	}
}

func TestCrawlerJobRegistryKeepsBrokenFile(t *testing.T) {
	dir := t.TempDir()
	useTempWorkingDir(t, dir)
	path := filepath.Join(dir, "data", "crawler-jobs.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := loadCrawlerJobs(); err == nil {
		t.Fatal("broken registry should return an error")
	}
	if data, _ := os.ReadFile(path); string(data) != "{not json" {
		t.Fatalf("loading must not rewrite the broken file, got %q", data)
	}
}

// A job is only "alive" while its PID belongs to a crawler process.
func TestCrawlerJobAliveGuardsPIDReuse(t *testing.T) {
	if _, err := os.Stat("/proc/self/cmdline"); err != nil {
		t.Skip("procfs not available")
	}

	notCrawler := exec.Command("sleep", "30")
	if err := notCrawler.Start(); err != nil {
		t.Skipf("cannot spawn sleep: %v", err)
	}
	defer func() {
		_ = notCrawler.Process.Kill()
		_, _ = notCrawler.Process.Wait()
	}()

	if jobAlive(notCrawler.Process.Pid) {
		t.Fatal("a non-crawler process must not count as a running job")
	}

	scriptDir := t.TempDir()
	script := filepath.Join(scriptDir, "treehole")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 30\n"), 0755); err != nil {
		t.Fatal(err)
	}
	crawlerLike := exec.Command(script, "crawler")
	if err := crawlerLike.Start(); err != nil {
		t.Fatalf("start fake crawler: %v", err)
	}
	defer func() {
		_ = crawlerLike.Process.Kill()
		_, _ = crawlerLike.Process.Wait()
	}()
	time.Sleep(50 * time.Millisecond)

	if !jobAlive(crawlerLike.Process.Pid) {
		t.Fatal("a running crawler process should be detected")
	}
	_ = crawlerLike.Process.Kill()
	_, _ = crawlerLike.Process.Wait()
	time.Sleep(50 * time.Millisecond)
	if jobAlive(crawlerLike.Process.Pid) {
		t.Fatal("a finished process must not be alive")
	}
}

func TestCrawlerJobStateAndProgress(t *testing.T) {
	dir := t.TempDir()
	useTempWorkingDir(t, dir)

	job := newCrawlerJob("监控", CrawlerParams{Mode: CrawlMonitor})
	job.PID = 999999
	if job.State() != JobFinished {
		t.Fatalf("state = %v, want finished for a dead PID", job.State())
	}
	job.PID = 0
	if job.State() != JobNeverStarted {
		t.Fatalf("state = %v, want never-started without a PID", job.State())
	}

	logPath, err := jobLogPath(job)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		t.Fatal(err)
	}
	content := "[Crawler] 第 1 页完成\nnoise\n[Crawler] 第 7 页完成\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if got := jobProgressLine(job); !strings.Contains(got, "第 7 页完成") {
		t.Fatalf("progress = %q, want the last [Crawler] line", got)
	}
}

// spawnFakeCrawler starts a real, harmless process whose command line contains
// "crawler" so jobAlive accepts it, without spawning the TUI binary.
func spawnFakeCrawler(t *testing.T) int {
	t.Helper()
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skipf("sleep not available: %v", err)
	}
	proc := exec.Command("sleep", "120")
	proc.Args = []string{"crawler", "120"}
	if err := proc.Start(); err != nil {
		t.Fatalf("start fake crawler: %v", err)
	}
	go func() { _ = proc.Wait() }()
	t.Cleanup(func() { _ = proc.Process.Kill() })
	return proc.Process.Pid
}

// installFakeJobLauncher makes startBackgroundJob use spawnFakeCrawler instead
// of the real TUI binary.
func installFakeJobLauncher(t *testing.T) {
	t.Helper()
	prev := launchJobProcess
	t.Cleanup(func() { launchJobProcess = prev })
	launchJobProcess = func(job CrawlerJob) (int, error) {
		return spawnFakeCrawler(t), nil
	}
}

// useTempWorkingDir points the app's runtime paths (data/) at a temp dir.
func useTempWorkingDir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}
