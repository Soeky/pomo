package cmd

import (
	"bytes"
	"database/sql"
	"io"
	"path/filepath"
	"testing"

	"github.com/Soeky/pomo/internal/config"
	"github.com/Soeky/pomo/internal/db"
	"github.com/spf13/cobra"
)

func TestCommandTUIStartupExitSmoke(t *testing.T) {
	t.Setenv("POMO_TUI_NO_RENDER", "1")
	opened := openTUITestDB(t)
	defer opened.Close()

	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("HOME", tmp)
	config.LoadConfig()

	testCases := []struct {
		name string
		run  func(cmd *cobra.Command) error
	}{
		{name: "event", run: runEventManagerTUI},
		{name: "plan", run: runSchedulerReviewTUI},
		{name: "config", run: runConfigWizardTUI},
	}

	for _, tc := range testCases {
		command := &cobra.Command{}
		command.SetIn(bytes.NewBufferString("q"))
		command.SetOut(io.Discard)
		command.SetErr(io.Discard)
		if err := tc.run(command); err != nil {
			t.Fatalf("%s TUI smoke run failed: %v", tc.name, err)
		}
	}
}

func openTUITestDB(t *testing.T) *sql.DB {
	t.Helper()

	opened, err := db.Open(filepath.Join(t.TempDir(), "pomo.db"))
	if err != nil {
		t.Fatalf("db.Open failed: %v", err)
	}
	prev := db.DB
	db.DB = opened
	t.Cleanup(func() { db.DB = prev })
	return opened
}
