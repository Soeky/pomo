package stats

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/db"
	"github.com/Soeky/pomo/internal/topics"
)

const DefaultAdherenceToleranceMinutes = 10

type PlanVsActualQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type OnTimeAdherenceSummary struct {
	PlannedCount      int
	OnTimeCount       int
	Percent           float64
	ToleranceMinutes  int
	ToleranceDuration time.Duration
}

type PlanCompletionSummary struct {
	PlannedCount int
	DoneCount    int
	Percent      float64
}

type DomainDriftRow struct {
	Domain           string
	ScheduledMinutes int
	ActualMinutes    int
	DriftMinutes     int
}

type DayBalanceRow struct {
	Day            string
	PlannedMinutes int
	ActualMinutes  int
	ScorePercent   float64
}

type WeeklyBalanceSummary struct {
	ScorePercent float64
	ActiveDays   int
	Days         []DayBalanceRow
}

type PlanVsActualMetrics struct {
	OnTimeAdherence OnTimeAdherenceSummary
	PlanCompletion  PlanCompletionSummary
	DriftByDomain   []DomainDriftRow
	WeeklyBalance   WeeklyBalanceSummary

	ScheduledMinutes int
	ActualMinutes    int
	DriftMinutes     int
}

type PlanVsActualReport struct {
	Label   string
	Metrics PlanVsActualMetrics
}

type AdherenceReport struct {
	Label   string
	Summary OnTimeAdherenceSummary
}

func BuildPlanVsActualReport(args []string, now time.Time) (PlanVsActualReport, error) {
	start, end, label := resolvePlanVsActualRange(args, now)
	metrics, err := QueryPlanVsActualMetrics(start, end, DefaultAdherenceToleranceMinutes)
	if err != nil {
		return PlanVsActualReport{}, err
	}
	return PlanVsActualReport{Label: label, Metrics: metrics}, nil
}

func BuildAdherenceReport(args []string, now time.Time) (AdherenceReport, error) {
	start, end, label := resolvePlanVsActualRange(args, now)
	metrics, err := QueryPlanVsActualMetrics(start, end, DefaultAdherenceToleranceMinutes)
	if err != nil {
		return AdherenceReport{}, err
	}
	return AdherenceReport{Label: label, Summary: metrics.OnTimeAdherence}, nil
}

func RenderAdherenceReport(report AdherenceReport) string {
	var b strings.Builder

	fmt.Fprintf(&b, "📅 Adherence (%s)\n\n", report.Label)
	fmt.Fprintf(&b, "Tolerance:      ±%d min\n", report.Summary.ToleranceMinutes)
	fmt.Fprintf(&b, "Planned blocks: %d\n", report.Summary.PlannedCount)
	fmt.Fprintf(&b, "On-time starts: %d\n", report.Summary.OnTimeCount)
	fmt.Fprintf(&b, "Adherence:      %.1f%%\n", report.Summary.Percent)

	return b.String()
}

func RenderPlanVsActualReport(report PlanVsActualReport) string {
	var b strings.Builder

	fmt.Fprintf(&b, "📅 Plan vs Actual (%s)\n\n", report.Label)
	fmt.Fprintf(&b, "On-time adherence: %.1f%% (%d/%d within ±%d min)\n",
		report.Metrics.OnTimeAdherence.Percent,
		report.Metrics.OnTimeAdherence.OnTimeCount,
		report.Metrics.OnTimeAdherence.PlannedCount,
		report.Metrics.OnTimeAdherence.ToleranceMinutes,
	)
	fmt.Fprintf(&b, "Plan completion:  %.1f%% (%d/%d done)\n",
		report.Metrics.PlanCompletion.Percent,
		report.Metrics.PlanCompletion.DoneCount,
		report.Metrics.PlanCompletion.PlannedCount,
	)
	fmt.Fprintf(&b, "Weekly balance:   %.1f%% (%d active days)\n",
		report.Metrics.WeeklyBalance.ScorePercent,
		report.Metrics.WeeklyBalance.ActiveDays,
	)

	b.WriteString("\nTotals:\n")
	fmt.Fprintf(&b, "Scheduled: %s h\n", FormatMinutesToHM(report.Metrics.ScheduledMinutes))
	fmt.Fprintf(&b, "Actual:    %s h\n", FormatMinutesToHM(report.Metrics.ActualMinutes))
	fmt.Fprintf(&b, "Drift:     %+d min\n", report.Metrics.DriftMinutes)

	b.WriteString("\nDrift by domain:\n")
	if len(report.Metrics.DriftByDomain) == 0 {
		b.WriteString("No planned or tracked focus data in range.\n")
		return b.String()
	}
	for _, row := range report.Metrics.DriftByDomain {
		fmt.Fprintf(&b, "- %-18s planned=%4dm actual=%4dm drift=%+4dm\n",
			row.Domain,
			row.ScheduledMinutes,
			row.ActualMinutes,
			row.DriftMinutes,
		)
	}

	return b.String()
}

func QueryPlanVsActualMetrics(start, end time.Time, toleranceMinutes int) (PlanVsActualMetrics, error) {
	return QueryPlanVsActualMetricsWithQuerier(context.Background(), db.DB, start, end, toleranceMinutes)
}

func QueryPlanVsActualMetricsWithQuerier(ctx context.Context, q PlanVsActualQuerier, start, end time.Time, toleranceMinutes int) (PlanVsActualMetrics, error) {
	if q == nil {
		return PlanVsActualMetrics{}, fmt.Errorf("database is not initialized")
	}
	if toleranceMinutes <= 0 {
		toleranceMinutes = DefaultAdherenceToleranceMinutes
	}
	if !end.After(start) {
		return PlanVsActualMetrics{}, fmt.Errorf("invalid range: end must be after start")
	}
	tolerance := time.Duration(toleranceMinutes) * time.Minute

	plannedRows, err := queryPlannedRows(ctx, q, start, end)
	if err != nil {
		return PlanVsActualMetrics{}, err
	}
	actualRows, err := queryFocusRows(ctx, q, start, end)
	if err != nil {
		return PlanVsActualMetrics{}, err
	}
	actualRowsForAdherence, err := queryFocusRows(ctx, q, start.Add(-tolerance), end.Add(tolerance))
	if err != nil {
		return PlanVsActualMetrics{}, err
	}

	plannedCount := len(plannedRows)
	doneCount := 0
	for _, row := range plannedRows {
		if strings.EqualFold(row.Status, "done") {
			doneCount++
		}
	}
	onTimeCount := computeOnTimeCount(plannedRows, actualRowsForAdherence, tolerance)

	scheduledSeconds := map[string]int{}
	actualSeconds := map[string]int{}
	plannedSecondsByDay := map[string]int{}
	actualSecondsByDay := map[string]int{}

	loc := start.Location()
	if loc == nil {
		loc = time.Local
	}

	totalScheduledSec := 0
	for _, row := range plannedRows {
		domain := normalizeDomain(row.Domain)
		scheduledSeconds[domain] += row.DurationSec
		totalScheduledSec += row.DurationSec

		dayKey := row.Start.In(loc).Format("2006-01-02")
		plannedSecondsByDay[dayKey] += row.DurationSec
	}

	totalActualSec := 0
	for _, row := range actualRows {
		domain := normalizeDomain(row.Domain)
		actualSeconds[domain] += row.DurationSec
		totalActualSec += row.DurationSec

		dayKey := row.Start.In(loc).Format("2006-01-02")
		actualSecondsByDay[dayKey] += row.DurationSec
	}

	domains := make([]string, 0, len(scheduledSeconds)+len(actualSeconds))
	seenDomain := map[string]struct{}{}
	for domain := range scheduledSeconds {
		seenDomain[domain] = struct{}{}
		domains = append(domains, domain)
	}
	for domain := range actualSeconds {
		if _, ok := seenDomain[domain]; ok {
			continue
		}
		domains = append(domains, domain)
	}

	driftRows := make([]DomainDriftRow, 0, len(domains))
	for _, domain := range domains {
		scheduledMin := scheduledSeconds[domain] / 60
		actualMin := actualSeconds[domain] / 60
		driftRows = append(driftRows, DomainDriftRow{
			Domain:           domain,
			ScheduledMinutes: scheduledMin,
			ActualMinutes:    actualMin,
			DriftMinutes:     actualMin - scheduledMin,
		})
	}
	sort.Slice(driftRows, func(i, j int) bool {
		leftAbs := absInt(driftRows[i].DriftMinutes)
		rightAbs := absInt(driftRows[j].DriftMinutes)
		if leftAbs != rightAbs {
			return leftAbs > rightAbs
		}
		if driftRows[i].Domain != driftRows[j].Domain {
			return driftRows[i].Domain < driftRows[j].Domain
		}
		if driftRows[i].ScheduledMinutes != driftRows[j].ScheduledMinutes {
			return driftRows[i].ScheduledMinutes > driftRows[j].ScheduledMinutes
		}
		return driftRows[i].ActualMinutes > driftRows[j].ActualMinutes
	})

	dayKeys := make([]string, 0, len(plannedSecondsByDay)+len(actualSecondsByDay))
	seenDay := map[string]struct{}{}
	for day := range plannedSecondsByDay {
		seenDay[day] = struct{}{}
		dayKeys = append(dayKeys, day)
	}
	for day := range actualSecondsByDay {
		if _, ok := seenDay[day]; ok {
			continue
		}
		dayKeys = append(dayKeys, day)
	}
	sort.Strings(dayKeys)

	dayRows := make([]DayBalanceRow, 0, len(dayKeys))
	totalDayScore := 0.0
	for _, day := range dayKeys {
		plannedMin := plannedSecondsByDay[day] / 60
		actualMin := actualSecondsByDay[day] / 60
		if plannedMin == 0 && actualMin == 0 {
			continue
		}
		score := dayBalanceScore(plannedMin, actualMin)
		dayRows = append(dayRows, DayBalanceRow{
			Day:            day,
			PlannedMinutes: plannedMin,
			ActualMinutes:  actualMin,
			ScorePercent:   score * 100,
		})
		totalDayScore += score
	}

	weeklyBalance := 0.0
	if len(dayRows) > 0 {
		weeklyBalance = totalDayScore / float64(len(dayRows)) * 100
	}

	metrics := PlanVsActualMetrics{
		OnTimeAdherence: OnTimeAdherenceSummary{
			PlannedCount:      plannedCount,
			OnTimeCount:       onTimeCount,
			Percent:           percentage(onTimeCount, plannedCount),
			ToleranceMinutes:  toleranceMinutes,
			ToleranceDuration: tolerance,
		},
		PlanCompletion: PlanCompletionSummary{
			PlannedCount: plannedCount,
			DoneCount:    doneCount,
			Percent:      percentage(doneCount, plannedCount),
		},
		DriftByDomain: driftRows,
		WeeklyBalance: WeeklyBalanceSummary{
			ScorePercent: weeklyBalance,
			ActiveDays:   len(dayRows),
			Days:         dayRows,
		},
		ScheduledMinutes: totalScheduledSec / 60,
		ActualMinutes:    totalActualSec / 60,
		DriftMinutes:     (totalActualSec / 60) - (totalScheduledSec / 60),
	}

	return metrics, nil
}

type plannedMetricRow struct {
	Domain      string
	Start       time.Time
	DurationSec int
	Status      string
}

type focusMetricRow struct {
	Domain      string
	Start       time.Time
	DurationSec int
}

func queryPlannedRows(ctx context.Context, q PlanVsActualQuerier, start, end time.Time) ([]plannedMetricRow, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT
			COALESCE(NULLIF(TRIM(domain), ''), 'General') AS domain,
			COALESCE(NULLIF(TRIM(title), ''), 'General') AS title,
			start_time,
			COALESCE(duration, 0) AS duration,
			COALESCE(status, 'planned') AS status
		FROM events
		WHERE layer = 'planned'
		  AND start_time >= ?
		  AND start_time < ?
		  AND COALESCE(status, 'planned') != 'canceled'
		ORDER BY start_time ASC, id ASC`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]plannedMetricRow, 0)
	for rows.Next() {
		var row plannedMetricRow
		var title string
		if err := rows.Scan(&row.Domain, &title, &row.Start, &row.DurationSec, &row.Status); err != nil {
			return nil, err
		}
		if row.DurationSec < 0 {
			row.DurationSec = 0
		}
		row.Domain = plannedDomain(row.Domain, title)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Start.Equal(out[j].Start) {
			if out[i].Domain != out[j].Domain {
				return out[i].Domain < out[j].Domain
			}
			return out[i].Status < out[j].Status
		}
		return out[i].Start.Before(out[j].Start)
	})

	return out, nil
}

func queryFocusRows(ctx context.Context, q PlanVsActualQuerier, start, end time.Time) ([]focusMetricRow, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT COALESCE(NULLIF(TRIM(domain), ''), COALESCE(NULLIF(TRIM(title), ''), 'General')), start_time, COALESCE(duration, 0)
		FROM events
		WHERE source = 'tracked'
		  AND layer = 'done'
		  AND kind = 'focus'
		  AND start_time >= ?
		  AND start_time < ?
		ORDER BY start_time ASC, id ASC`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]focusMetricRow, 0)
	for rows.Next() {
		var row focusMetricRow
		var topic string
		if err := rows.Scan(&topic, &row.Start, &row.DurationSec); err != nil {
			return nil, err
		}
		if row.DurationSec < 0 {
			row.DurationSec = 0
		}
		row.Domain = sessionDomain(topic)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func computeOnTimeCount(planned []plannedMetricRow, actual []focusMetricRow, tolerance time.Duration) int {
	plannedByDomain := map[string][]time.Time{}
	for _, row := range planned {
		domain := normalizeDomain(row.Domain)
		plannedByDomain[domain] = append(plannedByDomain[domain], row.Start)
	}
	actualByDomain := map[string][]time.Time{}
	for _, row := range actual {
		domain := normalizeDomain(row.Domain)
		actualByDomain[domain] = append(actualByDomain[domain], row.Start)
	}

	onTime := 0
	for domain, plannedStarts := range plannedByDomain {
		sort.Slice(plannedStarts, func(i, j int) bool { return plannedStarts[i].Before(plannedStarts[j]) })
		actualStarts := actualByDomain[domain]
		sort.Slice(actualStarts, func(i, j int) bool { return actualStarts[i].Before(actualStarts[j]) })

		pi, ai := 0, 0
		for pi < len(plannedStarts) && ai < len(actualStarts) {
			plannedStart := plannedStarts[pi]
			actualStart := actualStarts[ai]
			if actualStart.Before(plannedStart.Add(-tolerance)) {
				ai++
				continue
			}
			if actualStart.After(plannedStart.Add(tolerance)) {
				pi++
				continue
			}

			onTime++
			pi++
			ai++
		}
	}

	return onTime
}

func dayBalanceScore(plannedMinutes, actualMinutes int) float64 {
	if plannedMinutes <= 0 || actualMinutes <= 0 {
		return 0
	}
	maxValue := math.Max(float64(plannedMinutes), float64(actualMinutes))
	if maxValue == 0 {
		return 0
	}
	delta := math.Abs(float64(actualMinutes - plannedMinutes))
	score := 1 - (delta / maxValue)
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func percentage(value, total int) float64 {
	if total <= 0 {
		return 0
	}
	return (float64(value) / float64(total)) * 100
}

func normalizeDomain(domain string) string {
	trimmed := strings.TrimSpace(domain)
	if trimmed == "" {
		return topics.DefaultDomain
	}
	return trimmed
}

func sessionDomain(topic string) string {
	path, err := topics.Parse(topic)
	if err == nil {
		return normalizeDomain(path.Domain)
	}
	trimmed := strings.TrimSpace(topic)
	if trimmed == "" {
		return topics.DefaultDomain
	}
	return trimmed
}

func plannedDomain(domain, title string) string {
	candidate := strings.TrimSpace(domain)
	if candidate == "" {
		candidate = strings.TrimSpace(title)
	}
	path, err := topics.Parse(candidate)
	if err == nil {
		return normalizeDomain(path.Domain)
	}
	return normalizeDomain(candidate)
}

func resolvePlanVsActualRange(args []string, now time.Time) (time.Time, time.Time, string) {
	if len(args) == 0 {
		start, end := getTimeRangeAt("week", now)
		return start, end, formatRangeNameAt("week", now)
	}
	return resolveRange(args, now)
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
