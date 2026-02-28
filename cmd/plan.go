package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/parse"
	"github.com/Soeky/pomo/internal/scheduler"
	"github.com/Soeky/pomo/internal/topics"
	"github.com/spf13/cobra"
)

var (
	planStatusFrom string
	planStatusTo   string
)

var (
	planGenerateFrom    string
	planGenerateTo      string
	planGenerateDryRun  bool
	planGenerateReplace bool
)

var (
	planTargetAddTitle       string
	planTargetAddDomain      string
	planTargetAddSubtopic    string
	planTargetAddCadence     string
	planTargetAddDuration    string
	planTargetAddHours       float64
	planTargetAddOccurrences int
	planTargetAddActive      bool
	planTargetListActiveOnly bool
	planTargetSetActiveValue bool
)

var (
	planConstraintSetWeekdays       string
	planConstraintSetDayStart       string
	planConstraintSetDayEnd         string
	planConstraintSetLunchStart     string
	planConstraintSetLunchDuration  int
	planConstraintSetDinnerStart    string
	planConstraintSetDinnerDuration int
	planConstraintSetMaxHoursPerDay int
	planConstraintSetTimezone       string
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

var planGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "run balanced scheduler generation for workload targets",
	Run: func(cmd *cobra.Command, args []string) {
		from, err := parsePlanTime(planGenerateFrom)
		if err != nil {
			fmt.Println("❌ invalid --from:", err)
			os.Exit(1)
		}
		to, err := parsePlanTime(planGenerateTo)
		if err != nil {
			fmt.Println("❌ invalid --to:", err)
			os.Exit(1)
		}

		result, err := scheduler.GenerateFromDB(context.Background(), scheduler.DBInput{
			From:                     from,
			To:                       to,
			Persist:                  !planGenerateDryRun,
			ReplaceExistingScheduler: planGenerateReplace,
		})
		if err != nil {
			fmt.Println("❌ scheduler generation failed:", err)
			os.Exit(1)
		}

		mode := "apply"
		if planGenerateDryRun {
			mode = "dry-run"
		}
		fmt.Printf("✅ scheduler %s complete: generated=%d diagnostics=%d\n", mode, len(result.Generated), len(result.Diagnostics))
		if result.RunID > 0 {
			fmt.Printf("Run ID: %d\n", result.RunID)
		}
		for _, summary := range result.Summaries {
			fmt.Printf("- target %d %s::%s (%s): required=%s fixed_deduction=%s existing_scheduler=%s generated=%s remaining=%s\n",
				summary.TargetID,
				summary.Domain,
				summary.Subtopic,
				summary.Cadence,
				formatSeconds(summary.RequiredSeconds),
				formatSeconds(summary.FixedDeductionSeconds),
				formatSeconds(summary.SchedulerExistingSeconds),
				formatSeconds(summary.GeneratedSeconds),
				formatSeconds(summary.RemainingSeconds),
			)
		}
		for _, diag := range result.Diagnostics {
			fmt.Printf("! [%s] %s (target=%d missing=%s)\n", strings.ToUpper(diag.Severity), diag.Message, diag.TargetID, formatSeconds(diag.MissingSeconds))
		}
		if hasErrorDiagnostics(result.Diagnostics) {
			os.Exit(1)
		}
	},
}

var planTargetCmd = &cobra.Command{
	Use:   "target",
	Short: "manage workload targets",
}

var planTargetAddCmd = &cobra.Command{
	Use:   "add",
	Short: "add a workload target",
	Run: func(cmd *cobra.Command, args []string) {
		targetSeconds, err := resolveTargetSeconds(planTargetAddDuration, planTargetAddHours)
		if err != nil {
			fmt.Println("❌ invalid duration/hours:", err)
			os.Exit(1)
		}
		if planTargetAddOccurrences > 0 && targetSeconds == 0 {
			fmt.Println("❌ --duration or --hours is required when --occurrences is set")
			os.Exit(1)
		}
		if planTargetAddOccurrences == 0 && targetSeconds == 0 {
			fmt.Println("❌ target duration is required via --duration or --hours")
			os.Exit(1)
		}

		id, err := scheduler.CreateWorkloadTarget(context.Background(), scheduler.WorkloadTarget{
			Title:             strings.TrimSpace(planTargetAddTitle),
			Domain:            planTargetAddDomain,
			Subtopic:          planTargetAddSubtopic,
			Cadence:           planTargetAddCadence,
			TargetSeconds:     targetSeconds,
			TargetOccurrences: planTargetAddOccurrences,
			Active:            planTargetAddActive,
		})
		if err != nil {
			fmt.Println("❌ create target failed:", err)
			os.Exit(1)
		}
		fmt.Printf("✅ workload target created (ID %d)\n", id)
	},
}

var planTargetListCmd = &cobra.Command{
	Use:   "list",
	Short: "list workload targets",
	Run: func(cmd *cobra.Command, args []string) {
		targets, err := scheduler.ListWorkloadTargets(context.Background(), planTargetListActiveOnly)
		if err != nil {
			fmt.Println("❌ list targets failed:", err)
			os.Exit(1)
		}
		if len(targets) == 0 {
			fmt.Println("No workload targets found.")
			return
		}
		for _, target := range targets {
			active := "inactive"
			if target.Active {
				active = "active"
			}
			requirement := formatSeconds(target.TargetSeconds)
			if target.TargetOccurrences > 0 {
				requirement = fmt.Sprintf("%dx @ %s", target.TargetOccurrences, formatSeconds(target.TargetSeconds))
			}
			fmt.Printf("%4d %-8s %-20s %-24s %-8s %s\n",
				target.ID, active, target.Cadence, requirement, target.Domain+"::"+target.Subtopic, target.Title)
		}
	},
}

var planTargetDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "delete a workload target",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
		if err != nil || id <= 0 {
			fmt.Println("❌ invalid target id")
			os.Exit(1)
		}
		if err := scheduler.DeleteWorkloadTarget(context.Background(), id); err != nil {
			fmt.Println("❌ delete target failed:", err)
			os.Exit(1)
		}
		fmt.Printf("✅ workload target deleted (ID %d)\n", id)
	},
}

var planTargetSetActiveCmd = &cobra.Command{
	Use:   "set-active <id>",
	Short: "activate or deactivate a workload target",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
		if err != nil || id <= 0 {
			fmt.Println("❌ invalid target id")
			os.Exit(1)
		}
		if err := scheduler.SetWorkloadTargetActive(context.Background(), id, planTargetSetActiveValue); err != nil {
			fmt.Println("❌ set target active failed:", err)
			os.Exit(1)
		}
		state := "inactive"
		if planTargetSetActiveValue {
			state = "active"
		}
		fmt.Printf("✅ workload target %d set to %s\n", id, state)
	},
}

var planConstraintCmd = &cobra.Command{
	Use:   "constraint",
	Short: "manage scheduler constraints",
}

var planConstraintShowCmd = &cobra.Command{
	Use:   "show",
	Short: "show active scheduler constraints",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := scheduler.LoadConstraintConfig(context.Background())
		if err != nil {
			fmt.Println("❌ load constraints failed:", err)
			os.Exit(1)
		}
		fmt.Printf("active_weekdays=%s\n", strings.Join(cfg.ActiveWeekdays, ","))
		fmt.Printf("day_start=%s\n", cfg.DayStart)
		fmt.Printf("day_end=%s\n", cfg.DayEnd)
		fmt.Printf("lunch_start=%s\n", cfg.LunchStart)
		fmt.Printf("lunch_duration_minutes=%d\n", cfg.LunchDurationMinutes)
		fmt.Printf("dinner_start=%s\n", cfg.DinnerStart)
		fmt.Printf("dinner_duration_minutes=%d\n", cfg.DinnerDurationMinutes)
		fmt.Printf("max_hours_per_day=%d\n", cfg.MaxHoursPerDay)
		fmt.Printf("timezone=%s\n", cfg.Timezone)
	},
}

var planConstraintSetCmd = &cobra.Command{
	Use:   "set",
	Short: "set scheduler constraints",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := scheduler.LoadConstraintConfig(context.Background())
		if err != nil {
			fmt.Println("❌ load constraints failed:", err)
			os.Exit(1)
		}
		if cmd.Flags().Changed("weekdays") {
			cfg.ActiveWeekdays = splitCSV(planConstraintSetWeekdays)
		}
		if cmd.Flags().Changed("day-start") {
			cfg.DayStart = strings.TrimSpace(planConstraintSetDayStart)
		}
		if cmd.Flags().Changed("day-end") {
			cfg.DayEnd = strings.TrimSpace(planConstraintSetDayEnd)
		}
		if cmd.Flags().Changed("lunch-start") {
			cfg.LunchStart = strings.TrimSpace(planConstraintSetLunchStart)
		}
		if cmd.Flags().Changed("lunch-duration") {
			cfg.LunchDurationMinutes = planConstraintSetLunchDuration
		}
		if cmd.Flags().Changed("dinner-start") {
			cfg.DinnerStart = strings.TrimSpace(planConstraintSetDinnerStart)
		}
		if cmd.Flags().Changed("dinner-duration") {
			cfg.DinnerDurationMinutes = planConstraintSetDinnerDuration
		}
		if cmd.Flags().Changed("max-hours-day") {
			cfg.MaxHoursPerDay = planConstraintSetMaxHoursPerDay
		}
		if cmd.Flags().Changed("timezone") {
			cfg.Timezone = strings.TrimSpace(planConstraintSetTimezone)
		}

		if err := scheduler.SaveConstraintConfig(context.Background(), cfg); err != nil {
			fmt.Println("❌ save constraints failed:", err)
			os.Exit(1)
		}
		fmt.Println("✅ scheduler constraints updated")
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

func resolveTargetSeconds(durationRaw string, hours float64) (int, error) {
	durationRaw = strings.TrimSpace(durationRaw)
	if durationRaw != "" {
		parsed, err := parse.ParseDurationFromArg(durationRaw)
		if err != nil || parsed <= 0 {
			return 0, fmt.Errorf("invalid duration")
		}
		return int(parsed.Seconds()), nil
	}
	if hours > 0 {
		return int((time.Duration(hours * float64(time.Hour))).Seconds()), nil
	}
	return 0, nil
}

func hasErrorDiagnostics(diagnostics []scheduler.Diagnostic) bool {
	for _, diag := range diagnostics {
		if strings.EqualFold(diag.Severity, "error") {
			return true
		}
	}
	return false
}

func formatSeconds(seconds int) string {
	if seconds <= 0 {
		return "0m"
	}
	duration := time.Duration(seconds) * time.Second
	hours := int(duration / time.Hour)
	minutes := int((duration % time.Hour) / time.Minute)
	if hours > 0 {
		if minutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}
		out = append(out, token)
	}
	return out
}

func init() {
	now := time.Now()
	planStatusCmd.Flags().StringVar(&planStatusFrom, "from", now.AddDate(0, 0, -7).Format("2006-01-02T15:04"), "range start")
	planStatusCmd.Flags().StringVar(&planStatusTo, "to", now.AddDate(0, 0, 1).Format("2006-01-02T15:04"), "range end")

	planGenerateCmd.Flags().StringVar(&planGenerateFrom, "from", now.Format("2006-01-02T15:04"), "generation window start")
	planGenerateCmd.Flags().StringVar(&planGenerateTo, "to", now.AddDate(0, 0, 7).Format("2006-01-02T15:04"), "generation window end")
	planGenerateCmd.Flags().BoolVar(&planGenerateDryRun, "dry-run", false, "compute schedule without writing events/schedule_runs")
	planGenerateCmd.Flags().BoolVar(&planGenerateReplace, "replace", true, "replace existing scheduler events in the target window before apply")

	planTargetAddCmd.Flags().StringVar(&planTargetAddTitle, "title", "", "target title (optional)")
	planTargetAddCmd.Flags().StringVar(&planTargetAddDomain, "domain", topics.DefaultDomain, "target domain")
	planTargetAddCmd.Flags().StringVar(&planTargetAddSubtopic, "subtopic", topics.DefaultSubtopic, "target subtopic")
	planTargetAddCmd.Flags().StringVar(&planTargetAddCadence, "cadence", "weekly", "cadence: daily|weekly|monthly")
	planTargetAddCmd.Flags().StringVar(&planTargetAddDuration, "duration", "", "duration per cadence (or per occurrence when --occurrences > 0)")
	planTargetAddCmd.Flags().Float64Var(&planTargetAddHours, "hours", 0, "duration in hours (alternative to --duration)")
	planTargetAddCmd.Flags().IntVar(&planTargetAddOccurrences, "occurrences", 0, "occurrence count per cadence")
	planTargetAddCmd.Flags().BoolVar(&planTargetAddActive, "active", true, "whether target is active")

	planTargetListCmd.Flags().BoolVar(&planTargetListActiveOnly, "active-only", false, "show only active targets")
	planTargetSetActiveCmd.Flags().BoolVar(&planTargetSetActiveValue, "active", true, "target active value")

	planConstraintSetCmd.Flags().StringVar(&planConstraintSetWeekdays, "weekdays", "", "active weekdays csv (e.g. mon,tue,wed,thu,fri)")
	planConstraintSetCmd.Flags().StringVar(&planConstraintSetDayStart, "day-start", "", "day start HH:MM")
	planConstraintSetCmd.Flags().StringVar(&planConstraintSetDayEnd, "day-end", "", "day end HH:MM")
	planConstraintSetCmd.Flags().StringVar(&planConstraintSetLunchStart, "lunch-start", "", "lunch start HH:MM")
	planConstraintSetCmd.Flags().IntVar(&planConstraintSetLunchDuration, "lunch-duration", 0, "lunch duration in minutes")
	planConstraintSetCmd.Flags().StringVar(&planConstraintSetDinnerStart, "dinner-start", "", "dinner start HH:MM")
	planConstraintSetCmd.Flags().IntVar(&planConstraintSetDinnerDuration, "dinner-duration", 0, "dinner duration in minutes")
	planConstraintSetCmd.Flags().IntVar(&planConstraintSetMaxHoursPerDay, "max-hours-day", 0, "max schedulable hours per active day")
	planConstraintSetCmd.Flags().StringVar(&planConstraintSetTimezone, "timezone", "", "timezone name (IANA) or Local")

	planCmd.AddCommand(planStatusCmd)
	planCmd.AddCommand(planGenerateCmd)
	planCmd.AddCommand(planTargetCmd)
	planCmd.AddCommand(planConstraintCmd)

	planTargetCmd.AddCommand(planTargetAddCmd)
	planTargetCmd.AddCommand(planTargetListCmd)
	planTargetCmd.AddCommand(planTargetDeleteCmd)
	planTargetCmd.AddCommand(planTargetSetActiveCmd)

	planConstraintCmd.AddCommand(planConstraintShowCmd)
	planConstraintCmd.AddCommand(planConstraintSetCmd)

	rootCmd.AddCommand(planCmd)
}
