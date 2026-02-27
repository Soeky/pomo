package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/db"
	"github.com/spf13/cobra"
)

var (
	planStatusFrom string
	planStatusTo   string
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "planning and scheduling workflows",
}

var planStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "show planned vs done counts in a date range",
	Run: func(cmd *cobra.Command, args []string) {
		from, err := parsePlanTime(planStatusFrom)
		if err != nil {
			fmt.Println("❌ invalid --from:", err)
			os.Exit(1)
		}
		to, err := parsePlanTime(planStatusTo)
		if err != nil {
			fmt.Println("❌ invalid --to:", err)
			os.Exit(1)
		}

		var planned, done, blocked, canceled int
		if err := db.DB.QueryRow(`
			SELECT
				COALESCE(SUM(CASE WHEN status='planned' THEN 1 ELSE 0 END), 0),
				COALESCE(SUM(CASE WHEN status='done' THEN 1 ELSE 0 END), 0),
				COALESCE(SUM(CASE WHEN status='blocked' THEN 1 ELSE 0 END), 0),
				COALESCE(SUM(CASE WHEN status='canceled' THEN 1 ELSE 0 END), 0)
			FROM events
			WHERE layer='planned' AND start_time >= ? AND start_time < ?`, from, to).
			Scan(&planned, &done, &blocked, &canceled); err != nil {
			fmt.Println("❌ plan status query failed:", err)
			os.Exit(1)
		}

		total := planned + done + blocked + canceled
		completion := 0.0
		if total > 0 {
			completion = float64(done) * 100 / float64(total)
		}

		fmt.Printf("Range: %s -> %s\n", from.Format("2006-01-02 15:04"), to.Format("2006-01-02 15:04"))
		fmt.Printf("Planned:  %d\n", planned)
		fmt.Printf("Done:     %d\n", done)
		fmt.Printf("Blocked:  %d\n", blocked)
		fmt.Printf("Canceled: %d\n", canceled)
		fmt.Printf("Completion: %.1f%%\n", completion)
	},
}

func parsePlanTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("missing value")
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	var parsed time.Time
	var err error
	for _, layout := range layouts {
		parsed, err = time.Parse(layout, raw)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("expected RFC3339 or 2006-01-02T15:04")
}

func init() {
	now := time.Now()
	planStatusCmd.Flags().StringVar(&planStatusFrom, "from", now.AddDate(0, 0, -7).Format("2006-01-02T15:04"), "range start")
	planStatusCmd.Flags().StringVar(&planStatusTo, "to", now.AddDate(0, 0, 1).Format("2006-01-02T15:04"), "range end")

	planCmd.AddCommand(planStatusCmd)
	rootCmd.AddCommand(planCmd)
}
