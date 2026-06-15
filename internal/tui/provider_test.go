package tui

import "testing"

func TestSplitPIDSearch(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantPID     int32
		wantKeyword string
	}{
		{name: "plain keyword", input: "course review", wantKeyword: "course review"},
		{name: "pid only", input: "#8123", wantPID: 8123},
		{name: "pid with keyword", input: "#8123 course review", wantPID: 8123, wantKeyword: "course review"},
		{name: "invalid pid falls back", input: "#abc course", wantKeyword: "#abc course"},
		{name: "non prefix hash falls back", input: "course #8123", wantKeyword: "course #8123"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotPID, gotKeyword := splitPIDSearch(tc.input)
			if gotPID != tc.wantPID {
				t.Fatalf("pid = %d, want %d", gotPID, tc.wantPID)
			}
			if gotKeyword != tc.wantKeyword {
				t.Fatalf("keyword = %q, want %q", gotKeyword, tc.wantKeyword)
			}
		})
	}
}

func TestParsePostListSearch(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantPID      int32
		wantKeyword  string
		wantIsFollow *bool
	}{
		{name: "empty", input: ""},
		{name: "follow only", input: ":follow", wantIsFollow: boolPtr(true)},
		{name: "follow with keyword", input: ":follow course review", wantKeyword: "course review", wantIsFollow: boolPtr(true)},
		{name: "follow with pid", input: "#8123 :follow", wantPID: 8123, wantIsFollow: boolPtr(true)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parsePostListSearch(tc.input)
			if got.pid != tc.wantPID {
				t.Fatalf("pid = %d, want %d", got.pid, tc.wantPID)
			}
			if got.keyword != tc.wantKeyword {
				t.Fatalf("keyword = %q, want %q", got.keyword, tc.wantKeyword)
			}
			switch {
			case got.isFollow == nil && tc.wantIsFollow == nil:
			case got.isFollow == nil || tc.wantIsFollow == nil:
				t.Fatalf("isFollow = %v, want %v", got.isFollow, tc.wantIsFollow)
			case *got.isFollow != *tc.wantIsFollow:
				t.Fatalf("isFollow = %v, want %v", *got.isFollow, *tc.wantIsFollow)
			}
		})
	}
}

func boolPtr(v bool) *bool {
	return &v
}
