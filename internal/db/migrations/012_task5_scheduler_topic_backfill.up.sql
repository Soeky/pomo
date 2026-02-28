-- Task 5 hardening:
-- - strengthen scheduler-oriented indexes
-- - refresh planned_events->events compatibility sync to use structured topic columns
-- - reconcile planned-backed canonical event rows from legacy source-of-truth

CREATE INDEX IF NOT EXISTS idx_planned_events_topic ON planned_events(domain, subtopic);
CREATE INDEX IF NOT EXISTS idx_events_workload_target_time ON events(workload_target_id, start_time, end_time) WHERE workload_target_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_events_source_status_time ON events(source, status, start_time);
CREATE INDEX IF NOT EXISTS idx_workload_targets_active_cadence_topic ON workload_targets(active, cadence, domain, subtopic);
CREATE INDEX IF NOT EXISTS idx_schedule_constraints_updated_at ON schedule_constraints(updated_at);
CREATE INDEX IF NOT EXISTS idx_schedule_run_events_run_action ON schedule_run_events(run_id, action);

DROP TRIGGER IF EXISTS trg_planned_events_to_events_insert;
DROP TRIGGER IF EXISTS trg_planned_events_to_events_update;

CREATE TRIGGER IF NOT EXISTS trg_planned_events_to_events_insert
AFTER INSERT ON planned_events
BEGIN
    INSERT OR IGNORE INTO events (
        kind, title, domain, subtopic, description,
        start_time, end_time, duration, timezone,
        layer, status, source,
        legacy_source, legacy_id,
        created_at, updated_at
    )
    VALUES (
        'task',
        NEW.title,
        COALESCE(NULLIF(TRIM(NEW.domain), ''), COALESCE(NULLIF(TRIM(NEW.title), ''), 'General')),
        COALESCE(NULLIF(TRIM(NEW.subtopic), ''), 'General'),
        NEW.description,
        NEW.start_time,
        NEW.end_time,
        CAST((julianday(NEW.end_time) - julianday(NEW.start_time)) * 86400 AS INTEGER),
        'Local',
        'planned',
        CASE
            WHEN NEW.status = 'done' THEN 'done'
            WHEN NEW.status = 'canceled' THEN 'canceled'
            ELSE 'planned'
        END,
        CASE
            WHEN NEW.source = 'scheduler' THEN 'scheduler'
            ELSE 'manual'
        END,
        'planned_events',
        NEW.id,
        COALESCE(NEW.created_at, NEW.start_time, CURRENT_TIMESTAMP),
        COALESCE(NEW.updated_at, NEW.end_time, NEW.start_time, CURRENT_TIMESTAMP)
    );

    UPDATE events
    SET kind = 'task',
        title = NEW.title,
        domain = COALESCE(NULLIF(TRIM(NEW.domain), ''), COALESCE(NULLIF(TRIM(NEW.title), ''), 'General')),
        subtopic = COALESCE(NULLIF(TRIM(NEW.subtopic), ''), 'General'),
        description = NEW.description,
        start_time = NEW.start_time,
        end_time = NEW.end_time,
        duration = CAST((julianday(NEW.end_time) - julianday(NEW.start_time)) * 86400 AS INTEGER),
        timezone = 'Local',
        layer = 'planned',
        status = CASE
            WHEN NEW.status = 'done' THEN 'done'
            WHEN NEW.status = 'canceled' THEN 'canceled'
            ELSE 'planned'
        END,
        source = CASE
            WHEN NEW.source = 'scheduler' THEN 'scheduler'
            ELSE 'manual'
        END,
        recurrence_rule_id = NULL,
        workload_target_id = NULL,
        metadata_json = NULL,
        created_at = COALESCE(NEW.created_at, NEW.start_time, CURRENT_TIMESTAMP),
        updated_at = COALESCE(NEW.updated_at, NEW.end_time, NEW.start_time, CURRENT_TIMESTAMP)
    WHERE legacy_source = 'planned_events'
      AND legacy_id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS trg_planned_events_to_events_update
AFTER UPDATE ON planned_events
BEGIN
    INSERT OR IGNORE INTO events (
        kind, title, domain, subtopic, description,
        start_time, end_time, duration, timezone,
        layer, status, source,
        legacy_source, legacy_id,
        created_at, updated_at
    )
    VALUES (
        'task',
        NEW.title,
        COALESCE(NULLIF(TRIM(NEW.domain), ''), COALESCE(NULLIF(TRIM(NEW.title), ''), 'General')),
        COALESCE(NULLIF(TRIM(NEW.subtopic), ''), 'General'),
        NEW.description,
        NEW.start_time,
        NEW.end_time,
        CAST((julianday(NEW.end_time) - julianday(NEW.start_time)) * 86400 AS INTEGER),
        'Local',
        'planned',
        CASE
            WHEN NEW.status = 'done' THEN 'done'
            WHEN NEW.status = 'canceled' THEN 'canceled'
            ELSE 'planned'
        END,
        CASE
            WHEN NEW.source = 'scheduler' THEN 'scheduler'
            ELSE 'manual'
        END,
        'planned_events',
        NEW.id,
        COALESCE(NEW.created_at, NEW.start_time, CURRENT_TIMESTAMP),
        COALESCE(NEW.updated_at, NEW.end_time, NEW.start_time, CURRENT_TIMESTAMP)
    );

    UPDATE events
    SET kind = 'task',
        title = NEW.title,
        domain = COALESCE(NULLIF(TRIM(NEW.domain), ''), COALESCE(NULLIF(TRIM(NEW.title), ''), 'General')),
        subtopic = COALESCE(NULLIF(TRIM(NEW.subtopic), ''), 'General'),
        description = NEW.description,
        start_time = NEW.start_time,
        end_time = NEW.end_time,
        duration = CAST((julianday(NEW.end_time) - julianday(NEW.start_time)) * 86400 AS INTEGER),
        timezone = 'Local',
        layer = 'planned',
        status = CASE
            WHEN NEW.status = 'done' THEN 'done'
            WHEN NEW.status = 'canceled' THEN 'canceled'
            ELSE 'planned'
        END,
        source = CASE
            WHEN NEW.source = 'scheduler' THEN 'scheduler'
            ELSE 'manual'
        END,
        recurrence_rule_id = NULL,
        workload_target_id = NULL,
        metadata_json = NULL,
        created_at = COALESCE(NEW.created_at, NEW.start_time, CURRENT_TIMESTAMP),
        updated_at = COALESCE(NEW.updated_at, NEW.end_time, NEW.start_time, CURRENT_TIMESTAMP)
    WHERE legacy_source = 'planned_events'
      AND legacy_id = NEW.id;
END;

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
    COALESCE(NULLIF(TRIM(p.domain), ''), COALESCE(NULLIF(TRIM(p.title), ''), 'General')) AS domain,
    COALESCE(NULLIF(TRIM(p.subtopic), ''), 'General') AS subtopic,
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
SET kind = 'task',
    title = p.title,
    domain = COALESCE(NULLIF(TRIM(p.domain), ''), COALESCE(NULLIF(TRIM(p.title), ''), 'General')),
    subtopic = COALESCE(NULLIF(TRIM(p.subtopic), ''), 'General'),
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
