package crawler

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"treehole/internal/models"
)

// 以下结构与 scripts/find_deleted_posts.go 的输出 JSON 保持兼容，
// 报告脚本（digest.py / plot_deleted_density.py）直接消费这些字段。

type DeletedPost struct {
	PID       int64 `json:"pid"`
	Timestamp int64 `json:"timestamp"`
}

type DeletedRange struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

type DeletedReport struct {
	Span         [2]int64       `json:"span"`
	RemotePosts  int64          `json:"remote_posts_in_span"`
	Deleted      int64          `json:"deleted"`
	PagesFetched int            `json:"pages_fetched"`
	Ranges       []DeletedRange `json:"ranges"`
	DeletedPosts []DeletedPost  `json:"deleted_posts"`
	GeneratedAt  string         `json:"generated_at"`
}

// DeletedTracker 在一次分页抓取过程中收集远端 pid/时间戳，
// 结束时计算已抓取区间内的缺口（被删帖）并与现有报告文件合并。
type DeletedTracker struct {
	path     string
	remoteTs map[int64]int64
	pages    int
}

func NewDeletedTracker(path string) *DeletedTracker {
	return &DeletedTracker{path: path, remoteTs: make(map[int64]int64)}
}

// Reset 清空累计状态，供 loop 模式下每轮重新统计。
func (t *DeletedTracker) Reset() {
	t.remoteTs = make(map[int64]int64)
	t.pages = 0
}

// RecordPage 记录一页抓取结果中的远端帖子（按 pid 去重，保留首次时间戳）。
func (t *DeletedTracker) RecordPage(posts []models.Post) {
	for i := range posts {
		pid := int64(posts[i].Pid)
		if _, ok := t.remoteTs[pid]; !ok {
			t.remoteTs[pid] = int64(posts[i].Timestamp)
		}
	}
	t.pages++
}

// Finalize 计算缺口、插值时间戳，并与现有文件合并后写回。
func (t *DeletedTracker) Finalize() error {
	if len(t.remoteTs) == 0 {
		return fmt.Errorf("未收集到任何远端帖子")
	}
	pids := make([]int64, 0, len(t.remoteTs))
	for pid := range t.remoteTs {
		pids = append(pids, pid)
	}
	sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })

	var gaps [][2]int64
	for i := 1; i < len(pids); i++ {
		if pids[i]-pids[i-1] > 1 {
			gaps = append(gaps, [2]int64{pids[i-1] + 1, pids[i] - 1})
		}
	}

	rep := DeletedReport{Span: [2]int64{pids[0], pids[len(pids)-1]}, PagesFetched: t.pages}
	for _, g := range gaps {
		rep.Ranges = append(rep.Ranges, DeletedRange{Start: g[0], End: g[1]})
		rep.Deleted += g[1] - g[0] + 1
		tsLo := t.remoteTs[g[0]-1]
		tsHi := t.remoteTs[g[1]+1]
		span := g[1] + 1 - (g[0] - 1)
		for pid := g[0]; pid <= g[1]; pid++ {
			ts := tsLo + (tsHi-tsLo)*int64(pid-(g[0]-1))/span
			rep.DeletedPosts = append(rep.DeletedPosts, DeletedPost{PID: pid, Timestamp: ts})
		}
	}
	rep.RemotePosts = int64(len(t.remoteTs))
	rep.GeneratedAt = time.Now().Format(time.RFC3339)

	if _, statErr := os.Stat(t.path); statErr == nil {
		old, err := loadDeletedReport(t.path)
		if err != nil {
			return fmt.Errorf("解析现有删帖文件失败（保留原文件不覆盖）: %w", err)
		}
		rep = mergeDeletedReports(rep, *old, t.remoteTs)
	}
	if err := writeDeletedReport(t.path, rep); err != nil {
		return err
	}
	log.Printf("[Deleted] 已写入 %s: remote=%d deleted=%d (in %d gaps) span=%d-%d",
		t.path, rep.RemotePosts, rep.Deleted, len(rep.Ranges), rep.Span[0], rep.Span[1])
	return nil
}

func loadDeletedReport(path string) (*DeletedReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rep DeletedReport
	if err := json.Unmarshal(data, &rep); err != nil {
		return nil, err
	}
	return &rep, nil
}

func writeDeletedReport(path string, rep DeletedReport) error {
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// mergeDeletedReports 将新扫描结果与旧报告取并集；新扫描中观测到存活的 pid
// 会从并集中剔除（对旧报告误报的自我纠错）。范围与计数从合并后的 pid 集合重建。
func mergeDeletedReports(newRep, old DeletedReport, newRemoteTs map[int64]int64) DeletedReport {
	deletedTs := make(map[int64]int64)
	for _, p := range old.DeletedPosts {
		deletedTs[p.PID] = p.Timestamp
	}
	for _, p := range newRep.DeletedPosts {
		deletedTs[p.PID] = p.Timestamp
	}
	for pid := range newRemoteTs {
		delete(deletedTs, pid)
	}

	pids := make([]int64, 0, len(deletedTs))
	for pid := range deletedTs {
		pids = append(pids, pid)
	}
	sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })

	merged := DeletedReport{
		GeneratedAt:  newRep.GeneratedAt,
		PagesFetched: old.PagesFetched + newRep.PagesFetched,
	}
	merged.Span = [2]int64{
		minInt64(old.Span[0], newRep.Span[0]),
		maxInt64(old.Span[1], newRep.Span[1]),
	}
	for i, pid := range pids {
		if i > 0 && pid == pids[i-1]+1 {
			merged.Ranges[len(merged.Ranges)-1].End = pid
		} else {
			merged.Ranges = append(merged.Ranges, DeletedRange{Start: pid, End: pid})
		}
		merged.DeletedPosts = append(merged.DeletedPosts, DeletedPost{PID: pid, Timestamp: deletedTs[pid]})
	}
	merged.Deleted = int64(len(pids))
	merged.RemotePosts = newRep.RemotePosts
	return merged
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
