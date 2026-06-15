package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCaptureSinkWritesRawAndFrameFiles(t *testing.T) {
	dir := t.TempDir()

	sink, err := NewCaptureSink(dir)
	if err != nil {
		t.Fatalf("NewCaptureSink() error = %v", err)
	}
	if sink == nil {
		t.Fatal("NewCaptureSink() returned nil sink")
	}
	defer sink.Close()

	writer := sink.OutputWriter(&strings.Builder{})
	if _, err := writer.Write([]byte("\x1b[31mhello\x1b[0m")); err != nil {
		t.Fatalf("writer.Write() error = %v", err)
	}

	sink.RecordFrame("\x1b[35mframe\x1b[0m\nline2")

	rawOutput, err := os.ReadFile(filepath.Join(dir, "raw-output.ansi"))
	if err != nil {
		t.Fatalf("ReadFile(raw-output.ansi) error = %v", err)
	}
	if string(rawOutput) != "\x1b[31mhello\x1b[0m" {
		t.Fatalf("raw-output.ansi = %q", string(rawOutput))
	}

	frameANSI, err := os.ReadFile(filepath.Join(dir, "current-frame.ansi"))
	if err != nil {
		t.Fatalf("ReadFile(current-frame.ansi) error = %v", err)
	}
	if string(frameANSI) != "\x1b[35mframe\x1b[0m\nline2" {
		t.Fatalf("current-frame.ansi = %q", string(frameANSI))
	}

	frameText, err := os.ReadFile(filepath.Join(dir, "current-frame.txt"))
	if err != nil {
		t.Fatalf("ReadFile(current-frame.txt) error = %v", err)
	}
	if string(frameText) != "frame\nline2" {
		t.Fatalf("current-frame.txt = %q", string(frameText))
	}
}

func TestNewCaptureSinkUsesEnvironmentVariable(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TREEHOLE_TUI_CAPTURE_DIR", dir)

	sink, err := NewCaptureSink("")
	if err != nil {
		t.Fatalf("NewCaptureSink() error = %v", err)
	}
	if sink == nil {
		t.Fatal("NewCaptureSink() returned nil sink")
	}
	defer sink.Close()

	if sink.dir != dir {
		t.Fatalf("sink.dir = %q, want %q", sink.dir, dir)
	}
}

func TestOutputWriterPreservesTTYFileDescriptor(t *testing.T) {
	dir := t.TempDir()

	sink, err := NewCaptureSink(dir)
	if err != nil {
		t.Fatalf("NewCaptureSink() error = %v", err)
	}
	if sink == nil {
		t.Fatal("NewCaptureSink() returned nil sink")
	}
	defer sink.Close()

	writer := sink.OutputWriter(os.Stdout)
	type fdWriter interface {
		Fd() uintptr
	}

	fdw, ok := writer.(fdWriter)
	if !ok {
		t.Fatal("OutputWriter(os.Stdout) should preserve Fd()")
	}
	if got, want := fdw.Fd(), os.Stdout.Fd(); got != want {
		t.Fatalf("writer.Fd() = %d, want %d", got, want)
	}
}
