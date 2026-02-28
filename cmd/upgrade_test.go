package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeUpgradeVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{in: "", want: "latest"},
		{in: "   ", want: "latest"},
		{in: "v2.0.0", want: "v2.0.0"},
		{in: " latest ", want: "latest"},
	}

	for _, tt := range tests {
		if got := normalizeUpgradeVersion(tt.in); got != tt.want {
			t.Fatalf("normalizeUpgradeVersion(%q): got=%q want=%q", tt.in, got, tt.want)
		}
	}
}

func TestBackupDatabaseFile(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "pomo.db")
	if err := os.WriteFile(dbPath, []byte("sqlite-data"), 0o600); err != nil {
		t.Fatalf("seed db file failed: %v", err)
	}

	backupPath, err := backupDatabaseFile(dbPath)
	if err != nil {
		t.Fatalf("backupDatabaseFile failed: %v", err)
	}
	if backupPath == dbPath {
		t.Fatalf("backup path should differ from source path")
	}
	data, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup file failed: %v", err)
	}
	if string(data) != "sqlite-data" {
		t.Fatalf("unexpected backup content: %q", string(data))
	}
}
