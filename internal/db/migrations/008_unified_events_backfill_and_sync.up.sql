-- Enforce one canonical event row per legacy row before adding unique index.
DELETE FROM events
WHERE legacy_source IS NOT NULL
  AND legacy_id IS NOT NULL
  AND id NOT IN (
    SELECT MIN(id)
    FROM events
    WHERE legacy_source IS NOT NULL
      AND legacy_id IS NOT NULL
    GROUP BY legacy_source, legacy_id
  );

CREATE UNIQUE INDEX IF NOT EXISTS idx_events_legacy_unique ON events(legacy_source, legacy_id);
CREATE INDEX IF NOT EXISTS idx_events_source_time ON events(source, start_time, end_time);
CREATE INDEX IF NOT EXISTS idx_events_status_time ON events(status, start_time);
CREATE INDEX IF NOT EXISTS idx_events_kind_time ON events(kind, start_time);

CREATE INDEX IF NOT EXISTS idx_recurrence_window ON recurrence_rules(active, start_date, end_date);
CREATE INDEX IF NOT EXISTS idx_workload_targets_active ON workload_targets(active, cadence);
CREATE INDEX IF NOT EXISTS idx_schedule_runs_status_started ON schedule_runs(status, started_at);
CREATE INDEX IF NOT EXISTS idx_schedule_run_events_event_action ON schedule_run_events(event_id, action);

UPDATE events
SET timezone = 'Local'
WHERE timezone IS NULL OR timezone = '';

-- Idempotent reconciliation backfill for legacy session rows.
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

-- Idempotent reconciliation backfill for legacy planned event rows.
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

-- Compatibility adapters: keep canonical events synced when legacy tables are mutated.
CREATE TRIGGER IF NOT EXISTS trg_sessions_to_events_insert
AFTER INSERT ON sessions
BEGIN
    INSERT OR IGNORE INTO events (
        kind, title, domain, subtopic, description,
        start_time, end_time, duration, timezone,
        layer, status, source,
        legacy_source, legacy_id,
        created_at, updated_at
    )
    VALUES (
        CASE WHEN NEW.type = 'break' THEN 'break' ELSE 'focus' END,
        CASE
            WHEN NEW.type = 'break' THEN 'Break'
            ELSE COALESCE(NULLIF(TRIM(NEW.topic), ''), 'General::General')
        END,
        CASE
            WHEN NEW.type = 'break' THEN 'Break'
            ELSE COALESCE(NULLIF(TRIM(NEW.topic), ''), 'General')
        END,
        'General',
        NULL,
        NEW.start_time,
        COALESCE(NEW.end_time, datetime(NEW.start_time, '+' || COALESCE(NEW.duration, 0) || ' seconds')),
        COALESCE(
            NEW.duration,
            CAST((julianday(COALESCE(NEW.end_time, datetime(NEW.start_time, '+' || COALESCE(NEW.duration, 0) || ' seconds'))) - julianday(NEW.start_time)) * 86400 AS INTEGER)
        ),
        'Local',
        'done',
        CASE WHEN NEW.end_time IS NULL THEN 'in_progress' ELSE 'done' END,
        'tracked',
        'sessions',
        NEW.id,
        COALESCE(NEW.created_at, NEW.start_time, CURRENT_TIMESTAMP),
        COALESCE(NEW.updated_at, NEW.end_time, NEW.start_time, CURRENT_TIMESTAMP)
    );

    UPDATE events
    SET kind = CASE WHEN NEW.type = 'break' THEN 'break' ELSE 'focus' END,
        title = CASE
            WHEN NEW.type = 'break' THEN 'Break'
            ELSE COALESCE(NULLIF(TRIM(NEW.topic), ''), 'General::General')
        END,
        domain = CASE
            WHEN NEW.type = 'break' THEN 'Break'
            ELSE COALESCE(NULLIF(TRIM(NEW.topic), ''), 'General')
        END,
        subtopic = 'General',
        description = NULL,
        start_time = NEW.start_time,
        end_time = COALESCE(NEW.end_time, datetime(NEW.start_time, '+' || COALESCE(NEW.duration, 0) || ' seconds')),
        duration = COALESCE(
            NEW.duration,
            CAST((julianday(COALESCE(NEW.end_time, datetime(NEW.start_time, '+' || COALESCE(NEW.duration, 0) || ' seconds'))) - julianday(NEW.start_time)) * 86400 AS INTEGER)
        ),
        timezone = 'Local',
        layer = 'done',
        status = CASE WHEN NEW.end_time IS NULL THEN 'in_progress' ELSE 'done' END,
        source = 'tracked',
        created_at = COALESCE(NEW.created_at, NEW.start_time, CURRENT_TIMESTAMP),
        updated_at = COALESCE(NEW.updated_at, NEW.end_time, NEW.start_time, CURRENT_TIMESTAMP)
    WHERE legacy_source = 'sessions'
      AND legacy_id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS trg_sessions_to_events_update
AFTER UPDATE ON sessions
BEGIN
    INSERT OR IGNORE INTO events (
        kind, title, domain, subtopic, description,
        start_time, end_time, duration, timezone,
        layer, status, source,
        legacy_source, legacy_id,
        created_at, updated_at
    )
    VALUES (
        CASE WHEN NEW.type = 'break' THEN 'break' ELSE 'focus' END,
        CASE
            WHEN NEW.type = 'break' THEN 'Break'
            ELSE COALESCE(NULLIF(TRIM(NEW.topic), ''), 'General::General')
        END,
        CASE
            WHEN NEW.type = 'break' THEN 'Break'
            ELSE COALESCE(NULLIF(TRIM(NEW.topic), ''), 'General')
        END,
        'General',
        NULL,
        NEW.start_time,
        COALESCE(NEW.end_time, datetime(NEW.start_time, '+' || COALESCE(NEW.duration, 0) || ' seconds')),
        COALESCE(
            NEW.duration,
            CAST((julianday(COALESCE(NEW.end_time, datetime(NEW.start_time, '+' || COALESCE(NEW.duration, 0) || ' seconds'))) - julianday(NEW.start_time)) * 86400 AS INTEGER)
        ),
        'Local',
        'done',
        CASE WHEN NEW.end_time IS NULL THEN 'in_progress' ELSE 'done' END,
        'tracked',
        'sessions',
        NEW.id,
        COALESCE(NEW.created_at, NEW.start_time, CURRENT_TIMESTAMP),
        COALESCE(NEW.updated_at, NEW.end_time, NEW.start_time, CURRENT_TIMESTAMP)
    );

    UPDATE events
    SET kind = CASE WHEN NEW.type = 'break' THEN 'break' ELSE 'focus' END,
        title = CASE
            WHEN NEW.type = 'break' THEN 'Break'
            ELSE COALESCE(NULLIF(TRIM(NEW.topic), ''), 'General::General')
        END,
        domain = CASE
            WHEN NEW.type = 'break' THEN 'Break'
            ELSE COALESCE(NULLIF(TRIM(NEW.topic), ''), 'General')
        END,
        subtopic = 'General',
        description = NULL,
        start_time = NEW.start_time,
        end_time = COALESCE(NEW.end_time, datetime(NEW.start_time, '+' || COALESCE(NEW.duration, 0) || ' seconds')),
        duration = COALESCE(
            NEW.duration,
            CAST((julianday(COALESCE(NEW.end_time, datetime(NEW.start_time, '+' || COALESCE(NEW.duration, 0) || ' seconds'))) - julianday(NEW.start_time)) * 86400 AS INTEGER)
        ),
        timezone = 'Local',
        layer = 'done',
        status = CASE WHEN NEW.end_time IS NULL THEN 'in_progress' ELSE 'done' END,
        source = 'tracked',
        created_at = COALESCE(NEW.created_at, NEW.start_time, CURRENT_TIMESTAMP),
        updated_at = COALESCE(NEW.updated_at, NEW.end_time, NEW.start_time, CURRENT_TIMESTAMP)
    WHERE legacy_source = 'sessions'
      AND legacy_id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS trg_sessions_to_events_delete
AFTER DELETE ON sessions
BEGIN
    DELETE FROM events
    WHERE legacy_source = 'sessions'
      AND legacy_id = OLD.id;
END;

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
        NEW.title,
        'General',
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
        domain = NEW.title,
        subtopic = 'General',
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
        NEW.title,
        'General',
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
        domain = NEW.title,
        subtopic = 'General',
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
        created_at = COALESCE(NEW.created_at, NEW.start_time, CURRENT_TIMESTAMP),
        updated_at = COALESCE(NEW.updated_at, NEW.end_time, NEW.start_time, CURRENT_TIMESTAMP)
    WHERE legacy_source = 'planned_events'
      AND legacy_id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS trg_planned_events_to_events_delete
AFTER DELETE ON planned_events
BEGIN
    DELETE FROM events
    WHERE legacy_source = 'planned_events'
      AND legacy_id = OLD.id;
END;
