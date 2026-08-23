package db

import (
	"strings"
	"testing"

	"treehole/internal/models"
)

func TestBuildBooleanFTSQuery(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"二食堂", `"二 食 堂"`},
		{"保研 清华", `"保 研" AND "清 华"`},
		{"保研 OR 考研", `("保 研" OR "考 研")`},
		{"食堂|开门", `("食 堂" OR "开 门")`},
		{"(保研 OR 考研) 清华", `("保 研" OR "考 研") AND "清 华"`},
		{"保研 -清华", `"保 研" NOT ("清 华")`},
		{"(a OR b) -c -d", `("a" OR "b") NOT ("c" OR "d")`},
		{"a b -c", `("a" AND "b") NOT ("c")`},
		{"gpa3.5", `"gpa3 5"`},
		{"  多空格   查询 ", `"多 空 格" AND "查 询"`},
	}
	for _, tc := range cases {
		got, ok := buildBooleanFTSQuery(tc.in)
		if !ok || got != tc.want {
			t.Errorf("buildBooleanFTSQuery(%q) = %q (ok=%v), want %q", tc.in, got, ok, tc.want)
		}
	}

	failCases := []string{
		"",
		"   ",
		"-清华",
		"-清华 -北大",
		"(保研 OR 考研",
		"!!! ???",
		strings.Repeat("词 ", 300),
	}
	for _, in := range failCases {
		if got, ok := buildBooleanFTSQuery(in); ok {
			t.Errorf("buildBooleanFTSQuery(%q) = %q, want failure", in, got)
		}
	}
}

func TestOrAliasKeepsQuotedPipe(t *testing.T) {
	got, ok := buildBooleanFTSQuery(`"a|b" c`)
	if !ok {
		t.Fatal("expected ok")
	}
	if !strings.Contains(got, `"a|b"`) && !strings.Contains(got, `"a b"`) {
		t.Fatalf("quoted pipe should stay phrase-like, got %q", got)
	}
}

func TestSplitPIDToken(t *testing.T) {
	pid, rest := splitPIDToken("#123 保研 清华")
	if pid == nil || *pid != 123 {
		t.Fatalf("pid = %v, want 123", pid)
	}
	if rest != "保研 清华" {
		t.Fatalf("rest = %q", rest)
	}
	pid, rest = splitPIDToken("保研 #notapid")
	if pid != nil || rest != "保研 #notapid" {
		t.Fatalf("non-numeric pid token must remain: pid=%v rest=%q", pid, rest)
	}
	pid, rest = splitPIDToken("#456")
	if pid == nil || *pid != 456 || rest != "" {
		t.Fatalf("pid-only: pid=%v rest=%q", pid, rest)
	}
}

func TestFTSBooleanSearchIntegration(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	requireFTS(t, database)

	posts := []models.Post{
		{Pid: 1, Text: "保研经验分享清华路线", Type: "text", Timestamp: 1000},
		{Pid: 2, Text: "考研经验分享北大路线", Type: "text", Timestamp: 2000},
		{Pid: 3, Text: "保研还是就业好纠结", Type: "text", Timestamp: 3000},
		{Pid: 4, Text: "食堂新菜很好吃", Type: "text", Timestamp: 4000},
		{Pid: 5, Text: "期末考试周复习攻略", Type: "text", Timestamp: 5000},
		{Pid: 6, Text: "考试安排出来了好紧张", Type: "text", Timestamp: 6000},
	}
	if err := database.SaveCrawlResult(posts, nil); err != nil {
		t.Fatalf("SaveCrawlResult: %v", err)
	}

	cases := []struct {
		query string
		pids  []int32
	}{
		{"保研 OR 考研", []int32{3, 2, 1}},
		{"经验 -清华", []int32{2}},
		{"(保研 OR 考研) 分享", []int32{2, 1}},
		{"保研 | 就业", []int32{3, 1}},
		{"#2 经验", []int32{2}},
		{"考试 -攻略", []int32{6}},
		{"考试 -攻略 -安排", nil},
	}
	for _, tc := range cases {
		got, err := database.SearchPostsCursor(tc.query, 0, 10, false)
		if err != nil {
			t.Fatalf("SearchPostsCursor(%q): %v", tc.query, err)
		}
		if len(got) != len(tc.pids) {
			t.Fatalf("query %q: got %v, want pids %v", tc.query, pids(got), tc.pids)
		}
		for i, p := range got {
			if p.Pid != tc.pids[i] {
				t.Fatalf("query %q: result[%d] = pid %d, want %d", tc.query, i, p.Pid, tc.pids[i])
			}
		}
	}
}

func TestFTSBooleanFallbackOnParseError(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()
	requireFTS(t, database)

	if err := database.SaveCrawlResult([]models.Post{
		{Pid: 1, Text: "包含括号的内容(测试", Type: "text", Timestamp: 1000},
	}, nil); err != nil {
		t.Fatalf("SaveCrawlResult: %v", err)
	}

	got, err := database.SearchPostsCursor("(测试", 0, 10, false)
	if err != nil {
		t.Fatalf("unbalanced paren should fall back to LIKE path: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("LIKE fallback results = %v, want [1]", pids(got))
	}
}
