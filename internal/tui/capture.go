package tui

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

var ansiControlPattern = regexp.MustCompile(`(?:\x1b_G.*?\x1b\\)|(?:\x1b(?:\[[0-?]*[ -/]*[@-~]|[@-Z\\-_]))`)

type CaptureSink struct {
	mu       sync.Mutex
	dir      string
	rawFile  *os.File
	frameRaw string
	frameTxt string
}

func NewCaptureSink(dir string) (*CaptureSink, error) {
	if dir == "" {
		dir = os.Getenv("TREEHOLE_TUI_CAPTURE_DIR")
	}
	if dir == "" {
		return nil, nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	rawPath := filepath.Join(dir, "raw-output.ansi")
	rawFile, err := os.OpenFile(rawPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}

	return &CaptureSink{
		dir:      dir,
		rawFile:  rawFile,
		frameRaw: filepath.Join(dir, "current-frame.ansi"),
		frameTxt: filepath.Join(dir, "current-frame.txt"),
	}, nil
}

func (c *CaptureSink) OutputWriter(dst io.Writer) io.Writer {
	if c == nil {
		return dst
	}
	if file, ok := dst.(*os.File); ok {
		return &ttyMirrorWriter{
			tty:   file,
			clone: c.rawFile,
		}
	}
	return io.MultiWriter(dst, c.rawFile)
}

func (c *CaptureSink) RecordFrame(frame string) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	_ = os.WriteFile(c.frameRaw, []byte(frame), 0644)
	_ = os.WriteFile(c.frameTxt, []byte(stripANSISequences(frame)), 0644)
}

func (c *CaptureSink) Close() error {
	if c == nil || c.rawFile == nil {
		return nil
	}
	return c.rawFile.Close()
}

func stripANSISequences(s string) string {
	return ansiControlPattern.ReplaceAllString(s, "")
}

type ttyMirrorWriter struct {
	tty   *os.File
	clone io.Writer
}

func (w *ttyMirrorWriter) Write(p []byte) (int, error) {
	n, err := w.tty.Write(p)
	if n > 0 {
		if _, cloneErr := w.clone.Write(p[:n]); cloneErr != nil && err == nil {
			err = cloneErr
		}
	}
	return n, err
}

func (w *ttyMirrorWriter) Read(p []byte) (int, error) {
	return 0, fs.ErrInvalid
}

func (w *ttyMirrorWriter) Close() error {
	return nil
}

func (w *ttyMirrorWriter) Fd() uintptr {
	return w.tty.Fd()
}
