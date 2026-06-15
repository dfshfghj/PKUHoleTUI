package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"treehole/internal/models"
)

// TestHelpPanelWriteRealFrame renders the help panel with representative state
// and writes the stripped frame to .out/current-frame.txt for visual inspection.
func TestHelpPanelWriteRealFrame(t *testing.T) {
	m := newTestModel()
	m.Page = PagePosts
	m.Width = 138
	m.Height = 30
	m.Dialog = DialogHelp
	m.Session.Mode = SessionModeOnline
	m.Session.CanWriteOnline = true
	m.Posts.CanWrite = true
	m.Posts.PostList = []models.Post{
		{Pid: 8314820, Text: "单身的洞友们愿意用孤寡多长时间换一份心仪的实习offer\n\n比如，孤寡三个月就是三个月内完全没有桃花", Timestamp: 1749995220, Reply: 0, Likenum: 0},
		{Pid: 8314819, Text: "突然想在雨中的未名湖游个泳", Timestamp: 1749995220, Reply: 0, Likenum: 0},
		{Pid: 8314818, Text: "感觉坚持不下去了，我要向生活投降了\n谁来接收一下俘虏", Timestamp: 1749995160, Reply: 0, Likenum: 0},
		{Pid: 8314817, Text: "求问，临八本科生，还在基础医学院，投稿单位需要写基础医学院吗？", Timestamp: 1749995160, Reply: 0, Likenum: 0},
		{Pid: 8314816, Text: "我来记录一点memtor的逆天发言", Timestamp: 1749995160, Reply: 0, Likenum: 0},
		{Pid: 8314815, Text: "yp 三吃 诸位怎么看", Timestamp: 1749995160, Reply: 0, Likenum: 0},
		{Pid: 8314814, Text: "来个人骂醒我，我是做题区", Timestamp: 1749995100, Reply: 0, Likenum: 0},
		{Pid: 8314813, Text: "恭喜这位医学生开始转码了", Timestamp: 1749995040, Reply: 2, Likenum: 0},
		{Pid: 8314812, Text: "知行计划面试的时间可以调整吗TVT，刚才发现和期末考试撞了", Timestamp: 1749995040, Reply: 0, Likenum: 0},
	}
	m.Posts.SelectedPostIdx = 2

	output := m.View()
	stripped := stripANSI(output)

	_, filename, _, _ := runtime.Caller(0)
	outDir := filepath.Join(filepath.Dir(filename), "../..", ".out")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", outDir, err)
	}
	outPath := filepath.Join(outDir, "current-frame.txt")
	if err := os.WriteFile(outPath, []byte(stripped), 0644); err != nil {
		t.Fatalf("write frame: %v", err)
	}

	// Verify help panel is present and aligned: check that at least one help-item
	// key appears on the same line as its description.
	lines := strings.Split(stripped, "\n")
	found := 0
	for _, line := range lines {
		if strings.Contains(line, "Enter") && strings.Contains(line, "打开详情") {
			found++
		}
	}
	if found == 0 {
		t.Fatalf("help panel missing Enter/打开详情 on same line in:\n%s", stripped)
	}
	t.Logf("wrote %s (%d lines)", outPath, len(lines))
}
