package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestHelpOutputGolden(t *testing.T) {
	t.Setenv("POMO_TUI_NO_RENDER", "1")

	helpCases := []struct {
		name string
		args []string
	}{
		{name: "root", args: []string{"--help"}},
		{name: "config", args: []string{"help", "config"}},
		{name: "event", args: []string{"help", "event"}},
		{name: "event_recur", args: []string{"help", "event", "recur"}},
		{name: "event_dep", args: []string{"help", "event", "dep"}},
		{name: "plan", args: []string{"help", "plan"}},
		{name: "workflow", args: []string{"help", "workflow"}},
		{name: "set", args: []string{"help", "set"}},
	}

	update := strings.EqualFold(strings.TrimSpace(os.Getenv("UPDATE_GOLDEN")), "1")
	for _, tc := range helpCases {
		got, err := executeRootForTest(tc.args...)
		if err != nil {
			t.Fatalf("help command failed for %s: %v\noutput:\n%s", tc.name, err, got)
		}
		path := filepath.Join("testdata", "help", tc.name+".golden")
		if update {
			if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
				t.Fatalf("write golden %s failed: %v", path, err)
			}
			continue
		}

		wantBytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read golden %s failed: %v", path, err)
		}
		want := normalizeCLIOutput(string(wantBytes))
		if got != want {
			t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", tc.name, got, want)
		}
	}
}

func TestHelpExamplesAreValid(t *testing.T) {
	t.Setenv("POMO_TUI_NO_RENDER", "1")

	examples := []struct {
		name string
		args []string
	}{
		{name: "start_topic", args: []string{"start", "50m", "Math::DiscreteProbability", "--help"}},
		{name: "start_escaped_delimiter", args: []string{"start", "Math\\::History::Week 1", "--help"}},
		{name: "config_get", args: []string{"config", "get", "default_focus", "--help"}},
		{name: "config_set", args: []string{"config", "set", "default_focus", "50", "--help"}},
		{name: "config_describe", args: []string{"config", "describe", "web_mode", "--help"}},
		{name: "set_alias", args: []string{"set", "default_focus", "50", "--help"}},
		{name: "event_add", args: []string{"event", "add", "--title", "Math study block", "--start", "2026-03-01T10:00", "--end", "2026-03-01T11:30", "--domain", "Math", "--subtopic", "Discrete", "--help"}},
		{name: "event_recur_add", args: []string{"event", "recur", "add", "--title", "Weekly Review", "--start", "2026-03-02T09:00", "--duration", "1h", "--freq", "weekly", "--byday", "MO,WE", "--domain", "Planning", "--subtopic", "General", "--help"}},
		{name: "event_dep_add", args: []string{"event", "dep", "add", "42", "41", "--required", "--help"}},
		{name: "event_dep_override", args: []string{"event", "dep", "override", "42", "--admin", "--reason", "manual validation done offline", "--help"}},
		{name: "plan_target_add", args: []string{"plan", "target", "add", "--domain", "Math", "--subtopic", "Discrete", "--cadence", "weekly", "--hours", "8", "--help"}},
		{name: "plan_constraint_set", args: []string{"plan", "constraint", "set", "--weekdays", "mon,tue,wed,thu,fri", "--day-start", "08:00", "--day-end", "22:00", "--lunch-start", "12:30", "--lunch-duration", "60", "--dinner-start", "19:00", "--dinner-duration", "60", "--max-hours-day", "8", "--timezone", "Local", "--help"}},
		{name: "plan_generate", args: []string{"plan", "generate", "--from", "2026-03-01T00:00", "--to", "2026-03-08T00:00", "--dry-run", "--help"}},
		{name: "help_workflow", args: []string{"help", "workflow"}},
	}

	for _, tc := range examples {
		out, err := executeRootForTest(tc.args...)
		if err != nil {
			t.Fatalf("example %s failed: %v\nargs=%q\noutput:\n%s", tc.name, err, strings.Join(tc.args, " "), out)
		}
	}
}

func executeRootForTest(args ...string) (string, error) {
	resetHelpFlagState(rootCmd)
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetIn(bytes.NewBuffer(nil))
	rootCmd.SetArgs(args)
	_, err := rootCmd.ExecuteC()
	return normalizeCLIOutput(buf.String()), err
}

func normalizeCLIOutput(v string) string {
	v = strings.ReplaceAll(v, "\r\n", "\n")
	return strings.TrimSpace(v) + "\n"
}

func resetHelpFlagState(cmd *cobra.Command) {
	if flag := cmd.Flags().Lookup("help"); flag != nil {
		_ = flag.Value.Set("false")
		flag.Changed = false
	}
	if flag := cmd.PersistentFlags().Lookup("help"); flag != nil {
		_ = flag.Value.Set("false")
		flag.Changed = false
	}
	for _, child := range cmd.Commands() {
		resetHelpFlagState(child)
	}
}
