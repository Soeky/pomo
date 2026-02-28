-- Reconcile legacy-backed event rows with current legacy table values.
-- Safe to rerun: removes orphan mappings, inserts missing mappings, and updates drifted rows.

DELETE FROM events
WHERE legacy_source = 'sessions'
  AND legacy_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1
    FROM sessions s
    WHERE s.id = events.legacy_id
  );

DELETE FROM events
WHERE legacy_source = 'planned_events'
  AND legacy_id IS NOT NULL
  AND NOT EXISTS (
    SELECT 1
    FROM planned_events p
    WHERE p.id = events.legacy_id
  );

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
FROM sessions s;

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
FROM planned_events p;

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
  AND events.legacy_id = s.id;

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
  AND events.legacy_id = p.id;
