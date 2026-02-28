-- Task 6 hardening:
-- - normalize dependency-blocking fields for existing rows
-- - add dependency/blocking-oriented indexes for reconciliation and scheduler scans

UPDATE events
SET blocked_override = 0
WHERE blocked_override IS NULL;

UPDATE events
SET blocked_reason = NULL
WHERE status != 'blocked';

CREATE INDEX IF NOT EXISTS idx_event_dependencies_required
ON event_dependencies(event_id, required, depends_on_event_id);

CREATE INDEX IF NOT EXISTS idx_event_dependencies_dep_required
ON event_dependencies(depends_on_event_id, required, event_id);

CREATE INDEX IF NOT EXISTS idx_events_status_override_time
ON events(status, blocked_override, start_time);
