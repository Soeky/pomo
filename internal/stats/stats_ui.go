package stats

import (
	"fmt"
)

func ShowStats(args []string) {
	rangeStr := "day"
	if len(args) > 0 {
		rangeStr = args[0]
	}

	start, end := GetTimeRange(rangeStr)

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

	var focusBlockCount, focusBlockTotal int
	var breakBlockCount, breakBlockTotal int
	for _, b := range blocks {
		mins := b.Duration / 60
		if b.Type == "focus" {
			focusBlockCount++
			focusBlockTotal += mins
		} else if b.Type == "break" {
			breakBlockCount++
			breakBlockTotal += mins
		}
	}

	fmt.Printf("📅 %s\n\n", FormatRangeName(rangeStr))

	fmt.Println("🍅 Work:")
	totalFocusDur := 0
	totalFocusCount := 0
	for _, entry := range focusStats {
		fmt.Printf("- %-10s %2dx – %s h\n", entry.Topic, entry.Count, FormatMinutesToHM(entry.TotalMinutes))
		totalFocusDur += entry.TotalMinutes
		totalFocusCount += entry.Count
	}
	if focusBlockCount > 0 {
		avg := float64(focusBlockTotal) / float64(focusBlockCount)
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
	fmt.Printf("Worktime:  %s h\n", FormatMinutesToHM(totalFocusDur))
	fmt.Printf("Breaktime: %s h\n", FormatMinutesToHM(breakStats.TotalMinutes))
	fmt.Println()
}
