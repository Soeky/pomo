package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/db"
)

var (
	ErrDependencyCycle     = errors.New("dependency cycle detected")
	ErrOverrideNotAllowed  = errors.New("dependency override requires admin flag")
	ErrDependencyIDInvalid = errors.New("dependency event ids must be positive")
)

type Dependency struct {
	EventID          int64
	DependsOnEventID int64
	Required         bool
	DependsOnTitle   string
	DependsOnStatus  string
}

type DependencyTransition struct {
	EventID     int64
	OldStatus   string
	NewStatus   string
	Reason      string
	ChangedAt   time.Time
	TriggeredBy string
}

type sqlQuerier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func AddDependency(ctx context.Context, eventID, dependsOnEventID int64, required bool) error {
	if eventID <= 0 || dependsOnEventID <= 0 {
		return ErrDependencyIDInvalid
	}
	if eventID == dependsOnEventID {
		return ErrDependencyCycle
	}
	if err := ensureEventExists(ctx, db.DB, eventID); err != nil {
		return err
	}
	if err := ensureEventExists(ctx, db.DB, dependsOnEventID); err != nil {
		return err
	}
	cycle, err := wouldIntroduceCycle(ctx, db.DB, eventID, dependsOnEventID)
	if err != nil {
		return err
	}
	if cycle {
		return ErrDependencyCycle
	}

	requiredInt := 0
	if required {
		requiredInt = 1
	}
	if _, err := db.DB.ExecContext(ctx, `
		INSERT INTO event_dependencies(event_id, depends_on_event_id, required, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(event_id, depends_on_event_id) DO UPDATE SET required = excluded.required`,
		eventID, dependsOnEventID, requiredInt, time.Now(),
	); err != nil {
		return err
	}

	ids, err := dependentClosure(ctx, db.DB, eventID)
	if err != nil {
		return err
	}
	_, err = reconcileBlockedStatuses(ctx, db.DB, ids, "dependency_add")
	return err
}

func DeleteDependency(ctx context.Context, eventID, dependsOnEventID int64) error {
	if eventID <= 0 || dependsOnEventID <= 0 {
		return ErrDependencyIDInvalid
	}
	if _, err := db.DB.ExecContext(ctx, `
		DELETE FROM event_dependencies
		WHERE event_id = ? AND depends_on_event_id = ?`, eventID, dependsOnEventID); err != nil {
		return err
	}

	ids, err := dependentClosure(ctx, db.DB, eventID)
	if err != nil {
		return err
	}
	_, err = reconcileBlockedStatuses(ctx, db.DB, ids, "dependency_delete")
	return err
}

func ListDependencies(ctx context.Context, eventID int64) ([]Dependency, error) {
	if eventID <= 0 {
		return nil, ErrDependencyIDInvalid
	}
	rows, err := db.DB.QueryContext(ctx, `
		SELECT
			d.event_id,
			d.depends_on_event_id,
			d.required,
			COALESCE(e.title, ''),
			CASE
				WHEN e.id IS NULL THEN 'missing'
				ELSE COALESCE(e.status, 'planned')
			END AS status
		FROM event_dependencies d
		LEFT JOIN events e ON e.id = d.depends_on_event_id
		WHERE d.event_id = ?
		ORDER BY d.depends_on_event_id ASC`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Dependency, 0)
	for rows.Next() {
		var row Dependency
		var requiredInt int
		if err := rows.Scan(
			&row.EventID,
			&row.DependsOnEventID,
			&requiredInt,
			&row.DependsOnTitle,
			&row.DependsOnStatus,
		); err != nil {
			return nil, err
		}
		row.Required = requiredInt == 1
		if strings.TrimSpace(row.DependsOnTitle) == "" {
			row.DependsOnTitle = fmt.Sprintf("event-%d", row.DependsOnEventID)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func SetDependencyOverride(ctx context.Context, eventID int64, enabled, admin bool, reason, origin string) error {
	if eventID <= 0 {
		return ErrDependencyIDInvalid
	}
	if !admin {
		return ErrOverrideNotAllowed
	}
	if strings.TrimSpace(origin) == "" {
		origin = "cli"
	}
	reason = strings.TrimSpace(reason)

	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	before, err := dependencyStateForEvent(ctx, tx, eventID)
	if err != nil {
		return err
	}

	value := 0
	if enabled {
		value = 1
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE events
		SET blocked_override = ?,
		    blocked_reason = NULL,
		    updated_at = ?
		WHERE id = ?`, value, time.Now(), eventID); err != nil {
		return err
	}

	ids, err := dependentClosure(ctx, tx, eventID)
	if err != nil {
		return err
	}
	if _, err := reconcileBlockedStatuses(ctx, tx, ids, "dependency_override"); err != nil {
		return err
	}

	after, err := dependencyStateForEvent(ctx, tx, eventID)
	if err != nil {
		return err
	}
	if reason != "" {
		after["note"] = reason
	}
	action := "dependency_override_disable"
	if enabled {
		action = "dependency_override_enable"
	}
	if err := insertAuditLog(ctx, tx, "event", eventID, action, before, after, origin); err != nil {
		return err
	}

	return tx.Commit()
}

func ReconcileBlockedStatuses(ctx context.Context, eventIDs []int64) ([]DependencyTransition, error) {
	return reconcileBlockedStatuses(ctx, db.DB, eventIDs, "manual_reconcile")
}

func ReconcileBlockedStatusesInWindow(ctx context.Context, from, to time.Time) ([]DependencyTransition, error) {
	return ReconcileBlockedStatusesInWindowTx(ctx, db.DB, from, to)
}

func ReconcileBlockedStatusesInWindowTx(ctx context.Context, q sqlQuerier, from, to time.Time) ([]DependencyTransition, error) {
	if !to.After(from) {
		return nil, fmt.Errorf("invalid reconciliation window")
	}
	rows, err := q.QueryContext(ctx, `
		SELECT id
		FROM events
		WHERE layer = 'planned'
		  AND start_time < ?
		  AND end_time > ?
		  AND status != 'canceled'
		ORDER BY id ASC`, to, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return reconcileBlockedStatuses(ctx, q, ids, "window_reconcile")
}

func ReconcileBlockedStatusesForEventAndDependents(ctx context.Context, eventID int64) ([]DependencyTransition, error) {
	if eventID <= 0 {
		return nil, ErrDependencyIDInvalid
	}
	ids, err := dependentClosure(ctx, db.DB, eventID)
	if err != nil {
		return nil, err
	}
	return reconcileBlockedStatuses(ctx, db.DB, ids, "event_update")
}

func reconcileBlockedStatuses(ctx context.Context, q sqlQuerier, eventIDs []int64, trigger string) ([]DependencyTransition, error) {
	normalizedIDs := uniqueSortedIDs(eventIDs)
	if len(normalizedIDs) == 0 {
		return nil, nil
	}

	candidates, err := loadDependencyCandidates(ctx, q, normalizedIDs)
	if err != nil {
		return nil, err
	}
	transitions := make([]DependencyTransition, 0)
	now := time.Now()
	for _, candidate := range candidates {
		missing, err := unresolvedDependencies(ctx, q, candidate.ID)
		if err != nil {
			return nil, err
		}

		newStatus := candidate.Status
		newReason := candidate.BlockedReason

		if !strings.EqualFold(candidate.Layer, "planned") || candidate.Status == "canceled" || candidate.Status == "done" {
			newReason = ""
		} else if candidate.BlockedOverride {
			newReason = ""
			if candidate.Status == "blocked" {
				newStatus = "planned"
			}
		} else if len(missing) > 0 {
			newStatus = "blocked"
			newReason = formatBlockedReason(missing)
		} else {
			newReason = ""
			if candidate.Status == "blocked" {
				newStatus = "planned"
			}
		}

		if candidate.Status == newStatus && candidate.BlockedReason == newReason {
			continue
		}

		var reasonArg any
		if strings.TrimSpace(newReason) == "" {
			reasonArg = nil
		} else {
			reasonArg = newReason
		}
		if _, err := q.ExecContext(ctx, `
			UPDATE events
			SET status = ?,
			    blocked_reason = ?,
			    updated_at = ?
			WHERE id = ?`, newStatus, reasonArg, now, candidate.ID); err != nil {
			return nil, err
		}

		transitions = append(transitions, DependencyTransition{
			EventID:     candidate.ID,
			OldStatus:   candidate.Status,
			NewStatus:   newStatus,
			Reason:      newReason,
			ChangedAt:   now,
			TriggeredBy: trigger,
		})
	}
	return transitions, nil
}

type dependencyCandidate struct {
	ID              int64
	Layer           string
	Status          string
	BlockedReason   string
	BlockedOverride bool
}

type unresolvedDependency struct {
	ID     int64
	Title  string
	Status string
}

func loadDependencyCandidates(ctx context.Context, q sqlQuerier, eventIDs []int64) ([]dependencyCandidate, error) {
	placeholders := make([]string, 0, len(eventIDs))
	args := make([]any, 0, len(eventIDs))
	for _, id := range eventIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	rows, err := q.QueryContext(ctx, `
		SELECT id, layer, status, COALESCE(blocked_reason, ''), COALESCE(blocked_override, 0)
		FROM events
		WHERE id IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY id ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]dependencyCandidate, 0, len(eventIDs))
	for rows.Next() {
		var row dependencyCandidate
		var overrideInt int
		if err := rows.Scan(&row.ID, &row.Layer, &row.Status, &row.BlockedReason, &overrideInt); err != nil {
			return nil, err
		}
		row.Layer = strings.ToLower(strings.TrimSpace(row.Layer))
		row.Status = strings.ToLower(strings.TrimSpace(row.Status))
		row.BlockedOverride = overrideInt == 1
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func unresolvedDependencies(ctx context.Context, q sqlQuerier, eventID int64) ([]unresolvedDependency, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT
			d.depends_on_event_id,
			COALESCE(e.title, ''),
			CASE
				WHEN e.id IS NULL THEN 'missing'
				ELSE COALESCE(e.status, 'planned')
			END AS dep_status
		FROM event_dependencies d
		LEFT JOIN events e ON e.id = d.depends_on_event_id
		WHERE d.event_id = ?
		  AND d.required = 1
		ORDER BY d.depends_on_event_id ASC`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	missing := make([]unresolvedDependency, 0)
	for rows.Next() {
		var dep unresolvedDependency
		if err := rows.Scan(&dep.ID, &dep.Title, &dep.Status); err != nil {
			return nil, err
		}
		dep.Status = strings.ToLower(strings.TrimSpace(dep.Status))
		if dep.Status == "done" {
			continue
		}
		if strings.TrimSpace(dep.Title) == "" {
			dep.Title = fmt.Sprintf("event-%d", dep.ID)
		}
		missing = append(missing, dep)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return missing, nil
}

func formatBlockedReason(missing []unresolvedDependency) string {
	if len(missing) == 0 {
		return ""
	}
	parts := make([]string, 0, len(missing))
	for _, dep := range missing {
		parts = append(parts, fmt.Sprintf("waiting on #%d %s (%s)", dep.ID, dep.Title, dep.Status))
	}
	return strings.Join(parts, "; ")
}

func ensureEventExists(ctx context.Context, q sqlQuerier, eventID int64) error {
	var exists int
	if err := q.QueryRowContext(ctx, `SELECT 1 FROM events WHERE id = ?`, eventID).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("event %d: %w", eventID, sql.ErrNoRows)
		}
		return err
	}
	return nil
}

func wouldIntroduceCycle(ctx context.Context, q sqlQuerier, eventID, dependsOnEventID int64) (bool, error) {
	var cycleID int64
	err := q.QueryRowContext(ctx, `
		WITH RECURSIVE walk(id) AS (
			SELECT ?
			UNION
			SELECT d.depends_on_event_id
			FROM event_dependencies d
			JOIN walk w ON w.id = d.event_id
		)
		SELECT id
		FROM walk
		WHERE id = ?
		LIMIT 1`, dependsOnEventID, eventID).Scan(&cycleID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return cycleID == eventID, nil
}

func dependentClosure(ctx context.Context, q sqlQuerier, eventID int64) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `
		WITH RECURSIVE dependents(id) AS (
			SELECT ?
			UNION
			SELECT d.event_id
			FROM event_dependencies d
			JOIN dependents dep ON dep.id = d.depends_on_event_id
		)
		SELECT DISTINCT id
		FROM dependents
		ORDER BY id ASC`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

func dependencyStateForEvent(ctx context.Context, q sqlQuerier, eventID int64) (map[string]any, error) {
	var status string
	var override int
	var reason sql.NullString
	if err := q.QueryRowContext(ctx, `
		SELECT status, COALESCE(blocked_override, 0), blocked_reason
		FROM events
		WHERE id = ?`, eventID).Scan(&status, &override, &reason); err != nil {
		return nil, err
	}
	state := map[string]any{
		"status":           status,
		"blocked_override": override == 1,
	}
	if reason.Valid {
		state["blocked_reason"] = reason.String
	}
	return state, nil
}

func insertAuditLog(ctx context.Context, q sqlQuerier, entityType string, entityID int64, action string, before, after any, origin string) error {
	beforeJSON, err := marshalMaybeJSON(before)
	if err != nil {
		return err
	}
	afterJSON, err := marshalMaybeJSON(after)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `
		INSERT INTO audit_log(entity_type, entity_id, action, before_json, after_json, changed_at, origin)
		VALUES(?, ?, ?, ?, ?, ?, ?)`,
		entityType, entityID, action, beforeJSON, afterJSON, time.Now(), origin)
	return err
}

func marshalMaybeJSON(v any) (sql.NullString, error) {
	if v == nil {
		return sql.NullString{}, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: string(b), Valid: true}, nil
}

func uniqueSortedIDs(input []int64) []int64 {
	seen := make(map[int64]struct{}, len(input))
	out := make([]int64, 0, len(input))
	for _, id := range input {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
