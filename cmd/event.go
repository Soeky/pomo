package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/events"
	"github.com/Soeky/pomo/internal/parse"
	"github.com/Soeky/pomo/internal/topics"
	"github.com/Soeky/pomo/internal/tui"
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

var (
	recurAddTitle      string
	recurAddStart      string
	recurAddDuration   string
	recurAddKind       string
	recurAddDomain     string
	recurAddSubtopic   string
	recurAddFreq       string
	recurAddInterval   int
	recurAddByDay      string
	recurAddByMonthDay int
	recurAddUntil      string
	recurAddTimezone   string
	recurAddActive     bool

	recurListActiveOnly bool

	recurEditTitle      string
	recurEditStart      string
	recurEditDuration   string
	recurEditKind       string
	recurEditDomain     string
	recurEditSubtopic   string
	recurEditFreq       string
	recurEditInterval   int
	recurEditByDay      string
	recurEditByMonthDay int
	recurEditUntil      string
	recurEditTimezone   string
	recurEditActive     bool

	recurExpandFrom   string
	recurExpandTo     string
	recurExpandRuleID int64
)

var (
	eventDepAddRequired    bool
	eventDepOverrideAdmin  bool
	eventDepOverrideClear  bool
	eventDepOverrideReason string
)

var eventCmd = &cobra.Command{
	Use:   "event",
	Short: "manage planned and done events in unified event storage",
	Long: `Run without subcommands to open the Bubble Tea event manager.
Subcommands remain available for non-interactive usage:
  event add|list
  event recur add|list|edit|delete|expand
  event dep add|list|delete|override

Examples:
  pomo event add --title "Math study block" --start 2026-03-01T10:00 --end 2026-03-01T11:30 --domain Math --subtopic Discrete
  pomo event recur add --title "Weekly Review" --start 2026-03-02T09:00 --duration 1h --freq weekly --byday MO,WE --domain Planning --subtopic General
  pomo event dep add 42 41 --required
  pomo event dep override 42 --admin --reason "completed prerequisite outside pomo"`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runEventManagerTUI(cmd); err != nil {
			fmt.Println("❌ event manager failed:", err)
			os.Exit(1)
		}
	},
}

var eventRecurCmd = &cobra.Command{
	Use:   "recur",
	Short: "manage recurring rules",
	Long: `Create and manage recurring rule templates, then expand them into concrete events.

Examples:
  pomo event recur add --title "Weekly Review" --start 2026-03-02T09:00 --duration 1h --freq weekly --byday MO,WE --domain Planning --subtopic General
  pomo event recur list --active-only
  pomo event recur edit 1 --interval 2 --title "Deep Review"
  pomo event recur expand --from 2026-03-01T00:00 --to 2026-03-31T23:59`,
}

var eventDepCmd = &cobra.Command{
	Use:   "dep",
	Short: "manage event dependencies and blocking overrides",
	Long: `Manage dependency edges between events and explicit blocking overrides.

Examples:
  pomo event dep add 42 41 --required
  pomo event dep list 42
  pomo event dep delete 42 41
  pomo event dep override 42 --admin --reason "manual validation done offline"`,
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
			rule := "-"
			if e.RecurrenceRuleID != nil {
				rule = strconv.FormatInt(*e.RecurrenceRuleID, 10)
			}
			line := fmt.Sprintf("%4d %-8s %-9s %-11s %-20s %s -> %s %s::%s rule=%s",
				e.ID,
				e.Kind,
				e.Layer,
				e.Status,
				e.Title,
				e.StartTime.Format("2006-01-02 15:04"),
				e.EndTime.Format("2006-01-02 15:04"),
				e.Domain,
				e.Subtopic,
				rule,
			)
			if strings.EqualFold(e.Status, "blocked") && strings.TrimSpace(e.BlockedReason) != "" {
				line += " blocked_reason=" + strconv.Quote(e.BlockedReason)
			}
			fmt.Println(line)
		}
		if len(rows) == 0 {
			fmt.Println("No events found in range.")
		}
	},
}

var eventDepAddCmd = &cobra.Command{
	Use:   "add <event-id> <depends-on-id>",
	Short: "add or update a dependency edge",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		eventID, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
		if err != nil || eventID <= 0 {
			fmt.Println("❌ invalid event id")
			os.Exit(1)
		}
		dependsOnID, err := strconv.ParseInt(strings.TrimSpace(args[1]), 10, 64)
		if err != nil || dependsOnID <= 0 {
			fmt.Println("❌ invalid depends-on id")
			os.Exit(1)
		}
		if err := events.AddDependency(context.Background(), eventID, dependsOnID, eventDepAddRequired); err != nil {
			fmt.Println("❌ add dependency failed:", err)
			os.Exit(1)
		}
		mode := "required"
		if !eventDepAddRequired {
			mode = "optional"
		}
		fmt.Printf("✅ dependency added: event %d -> %d (%s)\n", eventID, dependsOnID, mode)
	},
}

var eventDepListCmd = &cobra.Command{
	Use:   "list <event-id>",
	Short: "list dependencies for an event",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		eventID, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
		if err != nil || eventID <= 0 {
			fmt.Println("❌ invalid event id")
			os.Exit(1)
		}
		eventRow, err := events.GetByID(context.Background(), eventID)
		if err != nil {
			fmt.Println("❌ load event failed:", err)
			os.Exit(1)
		}
		fmt.Printf("Event %d: %s (%s)\n", eventRow.ID, eventRow.Title, eventRow.Status)
		if strings.EqualFold(eventRow.Status, "blocked") && strings.TrimSpace(eventRow.BlockedReason) != "" {
			fmt.Printf("Blocked reason: %s\n", eventRow.BlockedReason)
		}
		rows, err := events.ListDependencies(context.Background(), eventID)
		if err != nil {
			fmt.Println("❌ list dependencies failed:", err)
			os.Exit(1)
		}
		if len(rows) == 0 {
			fmt.Println("No dependencies found.")
			return
		}
		for _, dep := range rows {
			mode := "required"
			if !dep.Required {
				mode = "optional"
			}
			fmt.Printf("- %d <- %d %s status=%s title=%s\n", dep.EventID, dep.DependsOnEventID, mode, dep.DependsOnStatus, dep.DependsOnTitle)
		}
	},
}

var eventDepDeleteCmd = &cobra.Command{
	Use:   "delete <event-id> <depends-on-id>",
	Short: "delete a dependency edge",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		eventID, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
		if err != nil || eventID <= 0 {
			fmt.Println("❌ invalid event id")
			os.Exit(1)
		}
		dependsOnID, err := strconv.ParseInt(strings.TrimSpace(args[1]), 10, 64)
		if err != nil || dependsOnID <= 0 {
			fmt.Println("❌ invalid depends-on id")
			os.Exit(1)
		}
		if err := events.DeleteDependency(context.Background(), eventID, dependsOnID); err != nil {
			fmt.Println("❌ delete dependency failed:", err)
			os.Exit(1)
		}
		fmt.Printf("✅ dependency deleted: event %d -/-> %d\n", eventID, dependsOnID)
	},
}

var eventDepOverrideCmd = &cobra.Command{
	Use:   "override <event-id>",
	Short: "enable or clear dependency blocking override (requires --admin)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		eventID, err := strconv.ParseInt(strings.TrimSpace(args[0]), 10, 64)
		if err != nil || eventID <= 0 {
			fmt.Println("❌ invalid event id")
			os.Exit(1)
		}
		enabled := !eventDepOverrideClear
		if err := events.SetDependencyOverride(context.Background(), eventID, enabled, eventDepOverrideAdmin, eventDepOverrideReason, "cli"); err != nil {
			fmt.Println("❌ set dependency override failed:", err)
			os.Exit(1)
		}
		if enabled {
			fmt.Printf("✅ dependency override enabled for event %d\n", eventID)
			return
		}
		fmt.Printf("✅ dependency override cleared for event %d\n", eventID)
	},
}

var eventRecurAddCmd = &cobra.Command{
	Use:   "add",
	Short: "add a recurring rule",
	Run: func(cmd *cobra.Command, args []string) {
		start, err := parseEventTime(recurAddStart)
		if err != nil {
			fmt.Println("❌ invalid --start:", err)
			os.Exit(1)
		}
		duration, err := parse.ParseDurationFromArg(strings.TrimSpace(recurAddDuration))
		if err != nil || duration <= 0 {
			fmt.Println("❌ invalid --duration: expected values like 25m, 1h, 1h30m")
			os.Exit(1)
		}
		path, err := topics.ParseParts(recurAddDomain, recurAddSubtopic)
		if err != nil {
			fmt.Println("❌ invalid topic path:", err)
			os.Exit(1)
		}
		byDays, err := parseWeekdayList(recurAddByDay)
		if err != nil {
			fmt.Println("❌ invalid --byday:", err)
			os.Exit(1)
		}
		rrule, err := events.BuildRRule(events.RecurrenceSpec{
			Freq:       recurAddFreq,
			Interval:   recurAddInterval,
			ByDays:     byDays,
			ByMonthDay: recurAddByMonthDay,
		})
		if err != nil {
			fmt.Println("❌ invalid recurrence spec:", err)
			os.Exit(1)
		}
		var untilPtr *time.Time
		if strings.TrimSpace(recurAddUntil) != "" {
			until, err := parseEventTime(recurAddUntil)
			if err != nil {
				fmt.Println("❌ invalid --until:", err)
				os.Exit(1)
			}
			untilPtr = &until
		}
		id, err := events.CreateRecurrenceRule(context.Background(), events.RecurrenceRule{
			Title:       strings.TrimSpace(recurAddTitle),
			Domain:      path.Domain,
			Subtopic:    path.Subtopic,
			Kind:        recurAddKind,
			DurationSec: int(duration.Seconds()),
			RRule:       rrule,
			Timezone:    strings.TrimSpace(recurAddTimezone),
			StartDate:   start,
			EndDate:     untilPtr,
			Active:      recurAddActive,
		})
		if err != nil {
			fmt.Println("❌ add recurrence rule failed:", err)
			os.Exit(1)
		}
		fmt.Printf("✅ recurrence rule created (ID %d): %s [%s::%s] %s\n", id, recurAddTitle, path.Domain, path.Subtopic, rrule)
	},
}

var eventRecurListCmd = &cobra.Command{
	Use:   "list",
	Short: "list recurring rules",
	Run: func(cmd *cobra.Command, args []string) {
		rules, err := events.ListRecurrenceRules(context.Background(), recurListActiveOnly)
		if err != nil {
			fmt.Println("❌ list recurrence rules failed:", err)
			os.Exit(1)
		}
		for _, rule := range rules {
			active := "inactive"
			if rule.Active {
				active = "active"
			}
			end := "-"
			if rule.EndDate != nil {
				end = rule.EndDate.Format("2006-01-02 15:04")
			}
			fmt.Printf("%4d %-8s %-8s %-20s %s::%s start=%s until=%s dur=%ds tz=%s rrule=%s\n",
				rule.ID, rule.Kind, active, rule.Title, rule.Domain, rule.Subtopic,
				rule.StartDate.Format("2006-01-02 15:04"), end, rule.DurationSec, rule.Timezone, rule.RRule)
		}
		if len(rules) == 0 {
			fmt.Println("No recurrence rules found.")
		}
	},
}

var eventRecurDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "delete a recurring rule",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil || id <= 0 {
			fmt.Println("❌ invalid recurrence rule id")
			os.Exit(1)
		}
		if err := events.DeleteRecurrenceRule(context.Background(), id); err != nil {
			fmt.Println("❌ delete recurrence rule failed:", err)
			os.Exit(1)
		}
		fmt.Printf("✅ recurrence rule deleted (ID %d)\n", id)
	},
}

var eventRecurEditCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "edit a recurring rule",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil || id <= 0 {
			fmt.Println("❌ invalid recurrence rule id")
			os.Exit(1)
		}
		rule, err := events.GetRecurrenceRuleByID(context.Background(), id)
		if err != nil {
			fmt.Println("❌ load recurrence rule failed:", err)
			os.Exit(1)
		}

		if cmd.Flags().Changed("title") {
			rule.Title = recurEditTitle
		}
		if cmd.Flags().Changed("start") {
			start, err := parseEventTime(recurEditStart)
			if err != nil {
				fmt.Println("❌ invalid --start:", err)
				os.Exit(1)
			}
			rule.StartDate = start
		}
		if cmd.Flags().Changed("duration") {
			duration, err := parse.ParseDurationFromArg(strings.TrimSpace(recurEditDuration))
			if err != nil || duration <= 0 {
				fmt.Println("❌ invalid --duration")
				os.Exit(1)
			}
			rule.DurationSec = int(duration.Seconds())
		}
		if cmd.Flags().Changed("kind") {
			rule.Kind = recurEditKind
		}
		if cmd.Flags().Changed("domain") || cmd.Flags().Changed("subtopic") {
			path, err := topics.ParseParts(defaultIfEmpty(recurEditDomain, rule.Domain), defaultIfEmpty(recurEditSubtopic, rule.Subtopic))
			if err != nil {
				fmt.Println("❌ invalid topic path:", err)
				os.Exit(1)
			}
			rule.Domain = path.Domain
			rule.Subtopic = path.Subtopic
		}
		if cmd.Flags().Changed("timezone") {
			rule.Timezone = recurEditTimezone
		}
		if cmd.Flags().Changed("active") {
			rule.Active = recurEditActive
		}
		if cmd.Flags().Changed("until") {
			if strings.TrimSpace(recurEditUntil) == "" {
				rule.EndDate = nil
			} else {
				until, err := parseEventTime(recurEditUntil)
				if err != nil {
					fmt.Println("❌ invalid --until:", err)
					os.Exit(1)
				}
				rule.EndDate = &until
			}
		}
		if cmd.Flags().Changed("freq") || cmd.Flags().Changed("interval") || cmd.Flags().Changed("byday") || cmd.Flags().Changed("bymonthday") {
			spec, err := events.ParseRRule(rule.RRule)
			if err != nil {
				fmt.Println("❌ parse existing recurrence spec failed:", err)
				os.Exit(1)
			}
			if cmd.Flags().Changed("freq") {
				spec.Freq = recurEditFreq
			}
			if cmd.Flags().Changed("interval") {
				spec.Interval = recurEditInterval
			}
			if cmd.Flags().Changed("byday") {
				days, err := parseWeekdayList(recurEditByDay)
				if err != nil {
					fmt.Println("❌ invalid --byday:", err)
					os.Exit(1)
				}
				spec.ByDays = days
			}
			if cmd.Flags().Changed("bymonthday") {
				spec.ByMonthDay = recurEditByMonthDay
			}
			rrule, err := events.BuildRRule(spec)
			if err != nil {
				fmt.Println("❌ invalid recurrence spec:", err)
				os.Exit(1)
			}
			rule.RRule = rrule
		}

		if err := events.UpdateRecurrenceRule(context.Background(), id, rule); err != nil {
			fmt.Println("❌ edit recurrence rule failed:", err)
			os.Exit(1)
		}
		fmt.Printf("✅ recurrence rule updated (ID %d)\n", id)
	},
}

var eventRecurExpandCmd = &cobra.Command{
	Use:   "expand",
	Short: "generate recurring occurrences into events in a date window",
	Run: func(cmd *cobra.Command, args []string) {
		from, err := parseEventTime(recurExpandFrom)
		if err != nil {
			fmt.Println("❌ invalid --from:", err)
			os.Exit(1)
		}
		to, err := parseEventTime(recurExpandTo)
		if err != nil {
			fmt.Println("❌ invalid --to:", err)
			os.Exit(1)
		}
		result, err := events.GenerateRecurringEventsInWindow(context.Background(), from, to, recurExpandRuleID)
		if err != nil {
			fmt.Println("❌ expand recurring events failed:", err)
			os.Exit(1)
		}
		fmt.Printf("✅ recurring expansion complete: rules=%d generated=%d skipped=%d\n", result.RulesProcessed, result.Generated, result.Skipped)
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
	eventCmd.AddCommand(eventRecurCmd)
	eventCmd.AddCommand(eventDepCmd)

	eventDepAddCmd.Flags().BoolVar(&eventDepAddRequired, "required", true, "whether dependency is required for unblocking")
	eventDepOverrideCmd.Flags().BoolVar(&eventDepOverrideAdmin, "admin", false, "confirm administrative override intent")
	eventDepOverrideCmd.Flags().BoolVar(&eventDepOverrideClear, "clear", false, "clear override instead of enabling")
	eventDepOverrideCmd.Flags().StringVar(&eventDepOverrideReason, "reason", "", "optional override reason for audit logging")

	eventDepCmd.AddCommand(eventDepAddCmd)
	eventDepCmd.AddCommand(eventDepListCmd)
	eventDepCmd.AddCommand(eventDepDeleteCmd)
	eventDepCmd.AddCommand(eventDepOverrideCmd)

	eventRecurAddCmd.Flags().StringVar(&recurAddTitle, "title", "", "rule title")
	eventRecurAddCmd.Flags().StringVar(&recurAddStart, "start", "", "first occurrence start time")
	eventRecurAddCmd.Flags().StringVar(&recurAddDuration, "duration", "", "occurrence duration (e.g. 45m, 1h30m)")
	eventRecurAddCmd.Flags().StringVar(&recurAddKind, "kind", "task", "event kind: focus|break|task|class|exercise|meal|other")
	eventRecurAddCmd.Flags().StringVar(&recurAddDomain, "domain", topics.DefaultDomain, "top-level topic domain")
	eventRecurAddCmd.Flags().StringVar(&recurAddSubtopic, "subtopic", topics.DefaultSubtopic, "topic subtopic")
	eventRecurAddCmd.Flags().StringVar(&recurAddFreq, "freq", "weekly", "frequency: daily|weekly|monthly")
	eventRecurAddCmd.Flags().IntVar(&recurAddInterval, "interval", 1, "repeat interval")
	eventRecurAddCmd.Flags().StringVar(&recurAddByDay, "byday", "", "weekly weekdays (MO,TU,WE,TH,FR,SA,SU)")
	eventRecurAddCmd.Flags().IntVar(&recurAddByMonthDay, "bymonthday", 0, "monthly day of month (1-31)")
	eventRecurAddCmd.Flags().StringVar(&recurAddUntil, "until", "", "optional inclusive end datetime")
	eventRecurAddCmd.Flags().StringVar(&recurAddTimezone, "timezone", "Local", "IANA timezone name or Local")
	eventRecurAddCmd.Flags().BoolVar(&recurAddActive, "active", true, "whether rule is active")
	_ = eventRecurAddCmd.MarkFlagRequired("title")
	_ = eventRecurAddCmd.MarkFlagRequired("start")
	_ = eventRecurAddCmd.MarkFlagRequired("duration")

	eventRecurListCmd.Flags().BoolVar(&recurListActiveOnly, "active-only", false, "show only active rules")

	eventRecurEditCmd.Flags().StringVar(&recurEditTitle, "title", "", "rule title")
	eventRecurEditCmd.Flags().StringVar(&recurEditStart, "start", "", "first occurrence start time")
	eventRecurEditCmd.Flags().StringVar(&recurEditDuration, "duration", "", "occurrence duration (e.g. 45m, 1h30m)")
	eventRecurEditCmd.Flags().StringVar(&recurEditKind, "kind", "", "event kind")
	eventRecurEditCmd.Flags().StringVar(&recurEditDomain, "domain", "", "top-level topic domain")
	eventRecurEditCmd.Flags().StringVar(&recurEditSubtopic, "subtopic", "", "topic subtopic")
	eventRecurEditCmd.Flags().StringVar(&recurEditFreq, "freq", "", "frequency: daily|weekly|monthly")
	eventRecurEditCmd.Flags().IntVar(&recurEditInterval, "interval", 1, "repeat interval")
	eventRecurEditCmd.Flags().StringVar(&recurEditByDay, "byday", "", "weekly weekdays")
	eventRecurEditCmd.Flags().IntVar(&recurEditByMonthDay, "bymonthday", 0, "monthly day of month (1-31)")
	eventRecurEditCmd.Flags().StringVar(&recurEditUntil, "until", "", "optional inclusive end datetime; pass empty to clear")
	eventRecurEditCmd.Flags().StringVar(&recurEditTimezone, "timezone", "", "IANA timezone name or Local")
	eventRecurEditCmd.Flags().BoolVar(&recurEditActive, "active", true, "whether rule is active")

	eventRecurExpandCmd.Flags().StringVar(&recurExpandFrom, "from", now.AddDate(0, 0, -1).Format("2006-01-02T15:04"), "window start")
	eventRecurExpandCmd.Flags().StringVar(&recurExpandTo, "to", now.AddDate(0, 1, 0).Format("2006-01-02T15:04"), "window end")
	eventRecurExpandCmd.Flags().Int64Var(&recurExpandRuleID, "rule-id", 0, "optional single recurrence rule id")

	eventRecurCmd.AddCommand(eventRecurAddCmd)
	eventRecurCmd.AddCommand(eventRecurListCmd)
	eventRecurCmd.AddCommand(eventRecurEditCmd)
	eventRecurCmd.AddCommand(eventRecurDeleteCmd)
	eventRecurCmd.AddCommand(eventRecurExpandCmd)

	rootCmd.AddCommand(eventCmd)
}

func parseWeekdayList(raw string) ([]time.Weekday, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	seen := map[time.Weekday]struct{}{}
	var out []time.Weekday
	for _, part := range parts {
		token := strings.ToUpper(strings.TrimSpace(part))
		switch token {
		case "MO", "MON", "MONDAY":
			if _, ok := seen[time.Monday]; !ok {
				out = append(out, time.Monday)
				seen[time.Monday] = struct{}{}
			}
		case "TU", "TUE", "TUESDAY":
			if _, ok := seen[time.Tuesday]; !ok {
				out = append(out, time.Tuesday)
				seen[time.Tuesday] = struct{}{}
			}
		case "WE", "WED", "WEDNESDAY":
			if _, ok := seen[time.Wednesday]; !ok {
				out = append(out, time.Wednesday)
				seen[time.Wednesday] = struct{}{}
			}
		case "TH", "THU", "THURSDAY":
			if _, ok := seen[time.Thursday]; !ok {
				out = append(out, time.Thursday)
				seen[time.Thursday] = struct{}{}
			}
		case "FR", "FRI", "FRIDAY":
			if _, ok := seen[time.Friday]; !ok {
				out = append(out, time.Friday)
				seen[time.Friday] = struct{}{}
			}
		case "SA", "SAT", "SATURDAY":
			if _, ok := seen[time.Saturday]; !ok {
				out = append(out, time.Saturday)
				seen[time.Saturday] = struct{}{}
			}
		case "SU", "SUN", "SUNDAY":
			if _, ok := seen[time.Sunday]; !ok {
				out = append(out, time.Sunday)
				seen[time.Sunday] = struct{}{}
			}
		default:
			return nil, fmt.Errorf("unsupported weekday token: %s", part)
		}
	}
	return out, nil
}

func defaultIfEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func runEventManagerTUI(cmd *cobra.Command) error {
	return tui.RunEventManager(tui.RunOptions{
		Input:      cmd.InOrStdin(),
		Output:     cmd.OutOrStdout(),
		NoRenderer: tui.NoRendererFromEnv(),
	})
}
