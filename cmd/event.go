package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/events"
	"github.com/Soeky/pomo/internal/topics"
	"github.com/spf13/cobra"
)

var (
	eventAddKind        string
	eventAddDomain      string
	eventAddSubtopic    string
	eventAddDescription string
	eventAddLayer       string
	eventAddStatus      string
	eventAddSource      string
)

var (
	eventListFrom string
	eventListTo   string
)

var eventCmd = &cobra.Command{
	Use:   "event",
	Short: "manage planned and done events in unified event storage",
}

var eventAddCmd = &cobra.Command{
	Use:   "add --title <title> --start <time> --end <time>",
	Short: "add a single event",
	Long: `Examples:
  pomo event add --title "Math study block" --start 2026-03-01T10:00 --end 2026-03-01T11:30 --domain "Math" --subtopic "Discrete Probability"
  pomo event add --title "Gym" --start 2026-03-01T18:00 --end 2026-03-01T20:00 --kind exercise --layer planned`,
	Run: func(cmd *cobra.Command, args []string) {
		title, _ := cmd.Flags().GetString("title")
		startRaw, _ := cmd.Flags().GetString("start")
		endRaw, _ := cmd.Flags().GetString("end")

		start, err := parseEventTime(startRaw)
		if err != nil {
			fmt.Println("❌ invalid --start:", err)
			os.Exit(1)
		}
		end, err := parseEventTime(endRaw)
		if err != nil {
			fmt.Println("❌ invalid --end:", err)
			os.Exit(1)
		}

		path, err := topics.ParseParts(eventAddDomain, eventAddSubtopic)
		if err != nil {
			fmt.Println("❌ invalid topic path:", err)
			os.Exit(1)
		}

		id, err := events.Create(context.Background(), events.Event{
			Kind:        eventAddKind,
			Title:       strings.TrimSpace(title),
			Domain:      path.Domain,
			Subtopic:    path.Subtopic,
			Description: eventAddDescription,
			StartTime:   start,
			EndTime:     end,
			Layer:       eventAddLayer,
			Status:      eventAddStatus,
			Source:      eventAddSource,
		})
		if err != nil {
			fmt.Println("❌ add event failed:", err)
			os.Exit(1)
		}
		fmt.Printf("✅ event created (ID %d): %s [%s::%s]\n", id, title, path.Domain, path.Subtopic)
	},
}

var eventListCmd = &cobra.Command{
	Use:   "list",
	Short: "list events in a time range",
	Run: func(cmd *cobra.Command, args []string) {
		from, err := parseEventTime(eventListFrom)
		if err != nil {
			fmt.Println("❌ invalid --from:", err)
			os.Exit(1)
		}
		to, err := parseEventTime(eventListTo)
		if err != nil {
			fmt.Println("❌ invalid --to:", err)
			os.Exit(1)
		}

		rows, err := events.ListInRange(context.Background(), from, to)
		if err != nil {
			fmt.Println("❌ list events failed:", err)
			os.Exit(1)
		}
		for _, e := range rows {
			fmt.Printf("%4d %-8s %-9s %-11s %-20s %s -> %s %s::%s\n",
				e.ID,
				e.Kind,
				e.Layer,
				e.Status,
				e.Title,
				e.StartTime.Format("2006-01-02 15:04"),
				e.EndTime.Format("2006-01-02 15:04"),
				e.Domain,
				e.Subtopic,
			)
		}
		if len(rows) == 0 {
			fmt.Println("No events found in range.")
		}
	},
}

func parseEventTime(raw string) (time.Time, error) {
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
	eventAddCmd.Flags().String("title", "", "event title")
	eventAddCmd.Flags().String("start", "", "start time (RFC3339 or 2006-01-02T15:04)")
	eventAddCmd.Flags().String("end", "", "end time (RFC3339 or 2006-01-02T15:04)")
	_ = eventAddCmd.MarkFlagRequired("title")
	_ = eventAddCmd.MarkFlagRequired("start")
	_ = eventAddCmd.MarkFlagRequired("end")

	eventAddCmd.Flags().StringVar(&eventAddKind, "kind", "task", "event kind: focus|break|task|class|exercise|meal|other")
	eventAddCmd.Flags().StringVar(&eventAddDomain, "domain", topics.DefaultDomain, "top-level topic domain")
	eventAddCmd.Flags().StringVar(&eventAddSubtopic, "subtopic", topics.DefaultSubtopic, "topic subtopic")
	eventAddCmd.Flags().StringVar(&eventAddDescription, "description", "", "optional description")
	eventAddCmd.Flags().StringVar(&eventAddLayer, "layer", "planned", "event layer: planned|done")
	eventAddCmd.Flags().StringVar(&eventAddStatus, "status", "planned", "event status")
	eventAddCmd.Flags().StringVar(&eventAddSource, "source", "manual", "event source")

	now := time.Now()
	eventListCmd.Flags().StringVar(&eventListFrom, "from", now.AddDate(0, 0, -1).Format("2006-01-02T15:04"), "range start")
	eventListCmd.Flags().StringVar(&eventListTo, "to", now.AddDate(0, 0, 7).Format("2006-01-02T15:04"), "range end")

	eventCmd.AddCommand(eventAddCmd)
	eventCmd.AddCommand(eventListCmd)
	rootCmd.AddCommand(eventCmd)
}
