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
		fmt.Println("Fehler bei Statistiken:", err)
		return
	}

	fmt.Printf("📅 %s\n\n", FormatRangeName(rangeStr))

	fmt.Println("🍅 Fokus:")
	totalFocusDur := 0
	totalFocusCount := 0
	for _, entry := range focusStats {
		fmt.Printf("- %-10s %2dx – %2d min\n", entry.Topic, entry.Count, entry.TotalMinutes)
		totalFocusDur += entry.TotalMinutes
		totalFocusCount += entry.Count
	}
	if totalFocusCount > 0 {
		avg := float64(totalFocusDur) / float64(totalFocusCount)
		fmt.Printf("Ø Fokuszeit: %.1f min\n", avg)
	} else {
		fmt.Println("Keine Fokuszeit.")
	}

	fmt.Println("\n💤 Pausen:")
	if breakStats.Count > 0 {
		fmt.Printf("- %dx – %d min\n", breakStats.Count, breakStats.TotalMinutes)
		avg := float64(breakStats.TotalMinutes) / float64(breakStats.Count)
		fmt.Printf("Ø Pausenzeit: %.1f min\n", avg)
	} else {
		fmt.Println("Keine Pausen.")
	}

	fmt.Println("\n🧠 Gesamt:")
	fmt.Printf("Fokuszeit:  %s h\n", FormatMinutesToHM(totalFocusDur))
	fmt.Printf("Pausenzeit: %s h\n", FormatMinutesToHM(breakStats.TotalMinutes))
	fmt.Println()
}
