package runtime

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDataAndDerivedPaths(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := DataDir()
	if !strings.Contains(dir, filepath.Join(".local", "share", "pomo")) {
		t.Fatalf("unexpected data dir: %s", dir)
	}
	if !strings.HasSuffix(PIDFilePath(), filepath.Join("pomo", "web.pid")) {
		t.Fatalf("unexpected pid path: %s", PIDFilePath())
	}
	if !strings.HasSuffix(StateFilePath(), filepath.Join("pomo", "web.state.json")) {
		t.Fatalf("unexpected state path: %s", StateFilePath())
	}
	if !strings.HasSuffix(LogFilePath(), filepath.Join("pomo", "web.log")) {
		t.Fatalf("unexpected log path: %s", LogFilePath())
	}
}
