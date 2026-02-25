package stats

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type WorkLine struct {
	Topic   string
	Count   int
	Minutes int
}

type StatsReport struct {
	Label        string
	Work         []WorkLine
	BreakCount   int
	BreakMinutes int
	WorkTotalMin int
	WorkAvgMin   float64
	HasWorkAvg   bool
	BreakAvgMin  float64
	HasBreakAvg  bool
}

func BuildReport(args []string, now time.Time) (StatsReport, error) {
	start, end, label := resolveRange(args, now)

	focusStats, breakStats, err := QueryStats(start, end)
	if err != nil {
		return StatsReport{}, err
	}
	blocks, err := QuerySessionBlocks(start, end)
	if err != nil {
		return StatsReport{}, err
	}

	report := StatsReport{
		Label:        label,
		BreakCount:   breakStats.Count,
		BreakMinutes: breakStats.TotalMinutes,
	}

	workBlockCount := 0
	workBlockTotal := 0
	breakBlockCount := 0
	breakBlockTotal := 0
	for _, b := range blocks {
		mins := b.Duration / 60
		switch b.Type {
		case "focus":
			workBlockCount++
			workBlockTotal += mins
		case "break":
			breakBlockCount++
			breakBlockTotal += mins
		}
	}

	for _, e := range focusStats {
		report.Work = append(report.Work, WorkLine{
			Topic:   e.Topic,
			Count:   e.Count,
			Minutes: e.TotalMinutes,
		})
		report.WorkTotalMin += e.TotalMinutes
	}

	if workBlockCount > 0 {
		report.WorkAvgMin = float64(workBlockTotal) / float64(workBlockCount)
		report.HasWorkAvg = true
	}
	if breakBlockCount > 0 {
		report.BreakAvgMin = float64(breakBlockTotal) / float64(breakBlockCount)
		report.HasBreakAvg = true
	}

	return report, nil
}

func RenderReport(report StatsReport) string {
	var b strings.Builder

	fmt.Fprintf(&b, "📅 %s\n\n", report.Label)
	b.WriteString("🍅 Work:\n")
	for _, line := range report.Work {
		fmt.Fprintf(&b, "- %-10s %2dx – %s h\n", line.Topic, line.Count, FormatMinutesToHM(line.Minutes))
	}
	if report.HasWorkAvg {
		fmt.Fprintf(&b, "Ø Worktime: %.1f min\n", report.WorkAvgMin)
	} else {
		b.WriteString("No worktime.\n")
	}

	b.WriteString("\n💤 Break:\n")
	if report.BreakCount > 0 {
		fmt.Fprintf(&b, "- %dx – %s h\n", report.BreakCount, FormatMinutesToHM(report.BreakMinutes))
		if report.HasBreakAvg {
			fmt.Fprintf(&b, "Ø Breaktime: %.1f min\n", report.BreakAvgMin)
		}
	} else {
		b.WriteString("No breaks.\n")
	}

	b.WriteString("\n🧠 Total:\n")
	fmt.Fprintf(&b, "Worktime:  %s h\n", FormatMinutesToHM(report.WorkTotalMin))
	fmt.Fprintf(&b, "Breaktime: %s h\n\n", FormatMinutesToHM(report.BreakMinutes))
	return b.String()
}

func resolveRange(args []string, now time.Time) (time.Time, time.Time, string) {
	var start, end time.Time
	var label string

	dateRE := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	monthRE := regexp.MustCompile(`^\d{4}-\d{2}$`)
	yearRE := regexp.MustCompile(`^\d{4}$`)

	switch len(args) {
	case 0:
		start, end = getTimeRangeAt("day", now)
		label = formatRangeNameAt("day", now)
	case 1:
		s := args[0]
		switch {
		case dateRE.MatchString(s):
			d, _ := time.Parse("2006-01-02", s)
			start = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
			end = start.AddDate(0, 0, 1)
			label = s
		case monthRE.MatchString(s):
			d, _ := time.Parse("2006-01", s)
			start = time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, now.Location())
			end = start.AddDate(0, 1, 0)
			label = s
		case yearRE.MatchString(s):
			y, _ := time.Parse("2006", s)
			start = time.Date(y.Year(), 1, 1, 0, 0, 0, 0, now.Location())
			end = start.AddDate(1, 0, 0)
			label = s
		default:
			start, end = getTimeRangeAt(s, now)
			label = formatRangeNameAt(s, now)
		}
	case 2:
		if dateRE.MatchString(args[0]) && dateRE.MatchString(args[1]) {
			s0, _ := time.Parse("2006-01-02", args[0])
			s1, _ := time.Parse("2006-01-02", args[1])
			start = time.Date(s0.Year(), s0.Month(), s0.Day(), 0, 0, 0, 0, now.Location())
			end = time.Date(s1.Year(), s1.Month(), s1.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)
			label = fmt.Sprintf("%s – %s", args[0], args[1])
		} else {
			start, end = getTimeRangeAt("day", now)
			label = formatRangeNameAt("day", now)
		}
	default:
		start, end = getTimeRangeAt("day", now)
		label = formatRangeNameAt("day", now)
	}

	return start, end, label
}
