package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const v2FinalizedMetaKey = "upgrade.v2_finalized"

type V2FinalizationResult struct {
	AlreadyFinalized         bool
	SessionsRows             int
	PlannedEventsRows        int
	SessionBackfilledRows    int
	PlannedBackfilledRows    int
	DroppedCompatibilitySync int
}

// FinalizeV2Cutover performs a one-time reconciliation from legacy tables into
// canonical events and then disables legacy sync triggers. This intentionally
// removes ongoing backward compatibility flows after migration parity is reached.
func FinalizeV2Cutover(ctx context.Context, opened *sql.DB) (V2FinalizationResult, error) {
	if opened == nil {
		return V2FinalizationResult{}, fmt.Errorf("database is not initialized")
	}

	tx, err := opened.BeginTx(ctx, nil)
	if err != nil {
		return V2FinalizationResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS app_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME NOT NULL
		)`); err != nil {
		return V2FinalizationResult{}, err
	}

	var existing string
	switch err := tx.QueryRowContext(ctx, `SELECT value FROM app_meta WHERE key = ?`, v2FinalizedMetaKey).Scan(&existing); err {
	case nil:
		if strings.EqualFold(strings.TrimSpace(existing), "true") {
			if err := tx.Commit(); err != nil {
				return V2FinalizationResult{}, err
			}
			return V2FinalizationResult{AlreadyFinalized: true}, nil
		}
	case sql.ErrNoRows:
	default:
		return V2FinalizationResult{}, err
	}

	result := V2FinalizationResult{}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM sessions`).Scan(&result.SessionsRows); err != nil {
		return V2FinalizationResult{}, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM planned_events`).Scan(&result.PlannedEventsRows); err != nil {
		return V2FinalizationResult{}, err
	}

	// Insert any missing legacy rows into events.
	res, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO events (
			kind, title, domain, subtopic, description,
			start_time, end_time, duration, timezone,
			layer, status, source,
			legacy_source, legacy_id,
			created_at, updated_at
		)
		SELECT
			CASE WHEN s.type = 'break' THEN 'break' ELSE 'focus' END AS kind,
			CASE
				WHEN s.type = 'break' THEN 'Break'
				ELSE COALESCE(NULLIF(TRIM(s.topic), ''), 'General::General')
			END AS title,
			CASE
				WHEN s.type = 'break' THEN 'Break'
				ELSE COALESCE(NULLIF(TRIM(s.topic), ''), 'General')
			END AS domain,
			'General' AS subtopic,
			NULL AS description,
			s.start_time AS start_time,
			COALESCE(s.end_time, datetime(s.start_time, '+' || COALESCE(s.duration, 0) || ' seconds')) AS end_time,
			COALESCE(
				s.duration,
				CAST((julianday(COALESCE(s.end_time, datetime(s.start_time, '+' || COALESCE(s.duration, 0) || ' seconds'))) - julianday(s.start_time)) * 86400 AS INTEGER)
			) AS duration,
			'Local' AS timezone,
			'done' AS layer,
			CASE WHEN s.end_time IS NULL THEN 'in_progress' ELSE 'done' END AS status,
			'tracked' AS source,
			'sessions' AS legacy_source,
			s.id AS legacy_id,
			COALESCE(s.created_at, s.start_time, CURRENT_TIMESTAMP) AS created_at,
			COALESCE(s.updated_at, s.end_time, s.start_time, CURRENT_TIMESTAMP) AS updated_at
		FROM sessions s`)
	if err != nil {
		return V2FinalizationResult{}, err
	}
	if affected, err := res.RowsAffected(); err == nil {
		result.SessionBackfilledRows = int(affected)
	}

	res, err = tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO events (
			kind, title, domain, subtopic, description,
			start_time, end_time, duration, timezone,
			layer, status, source,
			legacy_source, legacy_id,
			created_at, updated_at
		)
		SELECT
			'task' AS kind,
			p.title AS title,
			p.title AS domain,
			'General' AS subtopic,
			p.description AS description,
			p.start_time AS start_time,
			p.end_time AS end_time,
			CAST((julianday(p.end_time) - julianday(p.start_time)) * 86400 AS INTEGER) AS duration,
			'Local' AS timezone,
			'planned' AS layer,
			CASE
				WHEN p.status = 'done' THEN 'done'
				WHEN p.status = 'canceled' THEN 'canceled'
				ELSE 'planned'
			END AS status,
			CASE
				WHEN p.source = 'scheduler' THEN 'scheduler'
				ELSE 'manual'
			END AS source,
			'planned_events' AS legacy_source,
			p.id AS legacy_id,
			COALESCE(p.created_at, p.start_time, CURRENT_TIMESTAMP) AS created_at,
			COALESCE(p.updated_at, p.end_time, p.start_time, CURRENT_TIMESTAMP) AS updated_at
		FROM planned_events p`)
	if err != nil {
		return V2FinalizationResult{}, err
	}
	if affected, err := res.RowsAffected(); err == nil {
		result.PlannedBackfilledRows = int(affected)
	}

	// Normalize mapped rows to legacy source of truth right before disabling sync.
	if _, err := tx.ExecContext(ctx, `
		UPDATE events
		SET kind = CASE WHEN s.type = 'break' THEN 'break' ELSE 'focus' END,
			title = CASE
				WHEN s.type = 'break' THEN 'Break'
				ELSE COALESCE(NULLIF(TRIM(s.topic), ''), 'General::General')
			END,
			domain = CASE
				WHEN s.type = 'break' THEN 'Break'
				ELSE COALESCE(NULLIF(TRIM(s.topic), ''), 'General')
			END,
			subtopic = 'General',
			description = NULL,
			start_time = s.start_time,
			end_time = COALESCE(s.end_time, datetime(s.start_time, '+' || COALESCE(s.duration, 0) || ' seconds')),
			duration = COALESCE(
				s.duration,
				CAST((julianday(COALESCE(s.end_time, datetime(s.start_time, '+' || COALESCE(s.duration, 0) || ' seconds'))) - julianday(s.start_time)) * 86400 AS INTEGER)
			),
			timezone = 'Local',
			layer = 'done',
			status = CASE WHEN s.end_time IS NULL THEN 'in_progress' ELSE 'done' END,
			source = 'tracked',
			recurrence_rule_id = NULL,
			workload_target_id = NULL,
			metadata_json = NULL,
			created_at = COALESCE(s.created_at, s.start_time, CURRENT_TIMESTAMP),
			updated_at = COALESCE(s.updated_at, s.end_time, s.start_time, CURRENT_TIMESTAMP)
		FROM sessions s
		WHERE events.legacy_source = 'sessions'
		  AND events.legacy_id = s.id`); err != nil {
		return V2FinalizationResult{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE events
		SET kind = 'task',
			title = p.title,
			domain = p.title,
			subtopic = 'General',
			description = p.description,
			start_time = p.start_time,
			end_time = p.end_time,
			duration = CAST((julianday(p.end_time) - julianday(p.start_time)) * 86400 AS INTEGER),
			timezone = 'Local',
			layer = 'planned',
			status = CASE
				WHEN p.status = 'done' THEN 'done'
				WHEN p.status = 'canceled' THEN 'canceled'
				ELSE 'planned'
			END,
			source = CASE
				WHEN p.source = 'scheduler' THEN 'scheduler'
				ELSE 'manual'
			END,
			recurrence_rule_id = NULL,
			workload_target_id = NULL,
			metadata_json = NULL,
			created_at = COALESCE(p.created_at, p.start_time, CURRENT_TIMESTAMP),
			updated_at = COALESCE(p.updated_at, p.end_time, p.start_time, CURRENT_TIMESTAMP)
		FROM planned_events p
		WHERE events.legacy_source = 'planned_events'
		  AND events.legacy_id = p.id`); err != nil {
		return V2FinalizationResult{}, err
	}

	compatibilityTriggers := []string{
		"trg_sessions_to_events_insert",
		"trg_sessions_to_events_update",
		"trg_sessions_to_events_delete",
		"trg_planned_events_to_events_insert",
		"trg_planned_events_to_events_update",
		"trg_planned_events_to_events_delete",
		"trg_sessions_to_events_insert_clear_linkage",
		"trg_sessions_to_events_update_clear_linkage",
		"trg_planned_events_to_events_insert_clear_linkage",
		"trg_planned_events_to_events_update_clear_linkage",
	}
	for _, trig := range compatibilityTriggers {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM sqlite_master WHERE type = 'trigger' AND name = ?`, trig).Scan(&exists); err != nil {
			return V2FinalizationResult{}, err
		}
		if exists > 0 {
			result.DroppedCompatibilitySync++
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TRIGGER IF EXISTS %s", trig)); err != nil {
			return V2FinalizationResult{}, err
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO app_meta(key, value, updated_at)
		VALUES(?, 'true', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, v2FinalizedMetaKey, time.Now()); err != nil {
		return V2FinalizationResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return V2FinalizationResult{}, err
	}
	return result, nil
}
