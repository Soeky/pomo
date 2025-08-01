package stats

import (
	"fmt"
	"regexp"
	"time"
)

func ShowStats(args []string) {
	var start, end time.Time
	var label string

	// Regex für Datums-Strings
	dateRE := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	monthRE := regexp.MustCompile(`^\d{4}-\d{2}$`)
	yearRE := regexp.MustCompile(`^\d{4}$`)
	now := time.Now()

	switch len(args) {
	case 0:
		// Default: heute
		start, end = GetTimeRange("day")
		label = FormatRangeName("day")

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
			start, end = GetTimeRange(s)
			label = FormatRangeName(s)
		}

	case 2:
		if dateRE.MatchString(args[0]) && dateRE.MatchString(args[1]) {
			s0, _ := time.Parse("2006-01-02", args[0])
			s1, _ := time.Parse("2006-01-02", args[1])
			start = time.Date(s0.Year(), s0.Month(), s0.Day(), 0, 0, 0, 0, now.Location())
			end = time.Date(s1.Year(), s1.Month(), s1.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)
			label = fmt.Sprintf("%s – %s", args[0], args[1])
		} else {
			start, end = GetTimeRange("day")
			label = FormatRangeName("day")
		}

	default:
		start, end = GetTimeRange("day")
		label = FormatRangeName("day")
	}

	focusStats, breakStats, err := QueryStats(start, end)
	if err != nil {
		fmt.Println("error at statistics:", err)
		return
	}
	blocks, err := QuerySessionBlocks(start, end)
	if err != nil {
		fmt.Println("error while calculating average sessions:", err)
		return
	}

	var workBlockCount, workBlockTotal int
	var breakBlockCount, breakBlockTotal int
	for _, b := range blocks {
		mins := b.Duration / 60
		if b.Type == "focus" {
			workBlockCount++
			workBlockTotal += mins
		} else if b.Type == "break" {
			breakBlockCount++
			breakBlockTotal += mins
		}
	}

	fmt.Printf("📅 %s\n\n", label)

	fmt.Println("🍅 Work:")
	totalWorkDur := 0
	for _, e := range focusStats {
		fmt.Printf("- %-10s %2dx – %s h\n", e.Topic, e.Count, FormatMinutesToHM(e.TotalMinutes))
		totalWorkDur += e.TotalMinutes
	}
	if workBlockCount > 0 {
		avg := float64(workBlockTotal) / float64(workBlockCount)
		fmt.Printf("Ø Worktime: %.1f min\n", avg)
	} else {
		fmt.Println("No worktime.")
	}

	fmt.Println("\n💤 Break:")
	if breakStats.Count > 0 {
		fmt.Printf("- %dx – %s h\n", breakStats.Count, FormatMinutesToHM(breakStats.TotalMinutes))
		if breakBlockCount > 0 {
			avg := float64(breakBlockTotal) / float64(breakBlockCount)
			fmt.Printf("Ø Breaktime: %.1f min\n", avg)
		}
	} else {
		fmt.Println("No breaks.")
	}

	fmt.Println("\n🧠 Total:")
	fmt.Printf("Worktime:  %s h\n", FormatMinutesToHM(totalWorkDur))
	fmt.Printf("Breaktime: %s h\n", FormatMinutesToHM(breakStats.TotalMinutes))
	fmt.Println()
}
