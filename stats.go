package main

import (
	"database/sql"
	"fmt"
	"time"
)

func showStats(args []string) {
	rangeStr := "day"
	if len(args) > 0 {
		rangeStr = args[0]
	}

	start, end := getTimeRange(rangeStr)

	focusStats, breakStats, err := queryStats(start, end)
	if err != nil {
		fmt.Println("Fehler bei Statistiken:", err)
		return
	}

	fmt.Printf("📅 %s\n\n", formatRangeName(rangeStr))

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
	fmt.Printf("Fokuszeit:  %s h\n", formatMinutesToHM(totalFocusDur))
	fmt.Printf("Pausenzeit: %s h\n", formatMinutesToHM(breakStats.TotalMinutes))

}

type FocusStat struct {
	Topic        string
	Count        int
	TotalMinutes int
}

type BreakStat struct {
	Count        int
	TotalMinutes int
}

func queryStats(start, end time.Time) ([]FocusStat, BreakStat, error) {
	rows, err := db.Query(`
        SELECT topic, COUNT(*), SUM(duration)
        FROM sessions
        WHERE type = 'focus' AND start_time BETWEEN ? AND ?
        GROUP BY topic
        ORDER BY SUM(duration) DESC
        LIMIT 10
    `, start, end)
	if err != nil {
		return nil, BreakStat{}, err
	}
	defer rows.Close()

	var focusStats []FocusStat
	for rows.Next() {
		var topic string
		var count int
		var durSec int
		rows.Scan(&topic, &count, &durSec)
		focusStats = append(focusStats, FocusStat{
			Topic:        topic,
			Count:        count,
			TotalMinutes: durSec / 60,
		})
	}

	// Breaks separat
	var breakStat BreakStat
	row := db.QueryRow(`
        SELECT COUNT(*), SUM(duration)
        FROM sessions
        WHERE type = 'break' AND start_time BETWEEN ? AND ?
    `, start, end)
	var durSec sql.NullInt64
	row.Scan(&breakStat.Count, &durSec)
	if durSec.Valid {
		breakStat.TotalMinutes = int(durSec.Int64) / 60
	}

	return focusStats, breakStat, nil
}
func getTimeRange(view string) (time.Time, time.Time) {
	now := time.Now()
	var start time.Time

	switch view {
	case "day":
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "week":
		offset := int(now.Weekday())
		if offset == 0 {
			offset = 7
		}
		start = now.AddDate(0, 0, -offset+1)
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, now.Location())
	case "month":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	case "year":
		start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	case "all":
		start = time.Date(2000, 1, 1, 0, 0, 0, 0, now.Location())
	default:
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}

	return start, now
}

func formatRangeName(view string) string {
	switch view {
	case "day":
		return "Heute"
	case "week":
		return "Diese Woche"
	case "month":
		return "Diesen Monat"
	case "year":
		return "Dieses Jahr"
	case "all":
		return "Alle Zeit"
	default:
		return view
	}
}
func formatMinutesToHM(mins int) string {
	h := mins / 60
	m := mins % 60
	return fmt.Sprintf("%02d:%02d", h, m)
}
