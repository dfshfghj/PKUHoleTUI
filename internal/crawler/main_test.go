package crawler

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "crawler-recovery-dumps")
	if err != nil {
		panic(err)
	}
	recoveryDumpDir = dir
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
