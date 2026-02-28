CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    kind TEXT NOT NULL CHECK(kind IN ('focus','break','task','class','exercise','meal','other')),
    title TEXT NOT NULL,
    domain TEXT NOT NULL DEFAULT 'General',
    subtopic TEXT NOT NULL DEFAULT 'General',
    description TEXT,
    start_time DATETIME NOT NULL,
    end_time DATETIME NOT NULL,
    duration INTEGER NOT NULL,
    layer TEXT NOT NULL CHECK(layer IN ('planned','done')),
    status TEXT NOT NULL CHECK(status IN ('planned','in_progress','done','canceled','blocked')),
    source TEXT NOT NULL CHECK(source IN ('manual','tracked','recurring','scheduler')),
    recurrence_rule_id INTEGER,
    workload_target_id INTEGER,
    metadata_json TEXT,
    legacy_source TEXT,
    legacy_id INTEGER,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_events_time ON events(start_time, end_time);
CREATE INDEX IF NOT EXISTS idx_events_layer_status ON events(layer, status);
CREATE INDEX IF NOT EXISTS idx_events_topic ON events(domain, subtopic);
CREATE INDEX IF NOT EXISTS idx_events_legacy ON events(legacy_source, legacy_id);

CREATE TABLE IF NOT EXISTS recurrence_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    domain TEXT NOT NULL DEFAULT 'General',
    subtopic TEXT NOT NULL DEFAULT 'General',
    kind TEXT NOT NULL CHECK(kind IN ('focus','break','task','class','exercise','meal','other')),
    duration INTEGER NOT NULL,
    rrule TEXT NOT NULL,
    timezone TEXT NOT NULL DEFAULT 'Local',
    start_date DATETIME NOT NULL,
    end_date DATETIME,
    active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1)),
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_recurrence_active ON recurrence_rules(active);

CREATE TABLE IF NOT EXISTS workload_targets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    domain TEXT NOT NULL,
    subtopic TEXT NOT NULL DEFAULT 'General',
    cadence TEXT NOT NULL CHECK(cadence IN ('daily','weekly','monthly')),
    target_seconds INTEGER NOT NULL DEFAULT 0,
    target_occurrences INTEGER NOT NULL DEFAULT 0,
    active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1)),
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_workload_targets_topic ON workload_targets(domain, subtopic);

CREATE TABLE IF NOT EXISTS schedule_constraints (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL UNIQUE,
    value_json TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS event_dependencies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id INTEGER NOT NULL,
    depends_on_event_id INTEGER NOT NULL,
    required INTEGER NOT NULL DEFAULT 1 CHECK(required IN (0,1)),
    created_at DATETIME NOT NULL,
    UNIQUE(event_id, depends_on_event_id)
);
CREATE INDEX IF NOT EXISTS idx_event_dependencies_event ON event_dependencies(event_id);
CREATE INDEX IF NOT EXISTS idx_event_dependencies_depends ON event_dependencies(depends_on_event_id);

CREATE TABLE IF NOT EXISTS schedule_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at DATETIME NOT NULL,
    finished_at DATETIME,
    status TEXT NOT NULL CHECK(status IN ('running','success','failed')),
    input_hash TEXT,
    error TEXT
);

CREATE TABLE IF NOT EXISTS schedule_run_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id INTEGER NOT NULL,
    event_id INTEGER NOT NULL,
    action TEXT NOT NULL CHECK(action IN ('create','update','skip','block')),
    details_json TEXT,
    created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_schedule_run_events_run ON schedule_run_events(run_id);
CREATE INDEX IF NOT EXISTS idx_schedule_run_events_event ON schedule_run_events(event_id);

INSERT INTO events (
    kind, title, domain, subtopic, description,
    start_time, end_time, duration,
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
    'done' AS layer,
    CASE WHEN s.end_time IS NULL THEN 'in_progress' ELSE 'done' END AS status,
    'tracked' AS source,
    'sessions' AS legacy_source,
    s.id AS legacy_id,
    COALESCE(s.created_at, s.start_time, CURRENT_TIMESTAMP) AS created_at,
    COALESCE(s.updated_at, s.end_time, s.start_time, CURRENT_TIMESTAMP) AS updated_at
FROM sessions s
WHERE NOT EXISTS (
    SELECT 1
    FROM events e
    WHERE e.legacy_source = 'sessions' AND e.legacy_id = s.id
);

INSERT INTO events (
    kind, title, domain, subtopic, description,
    start_time, end_time, duration,
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
FROM planned_events p
WHERE NOT EXISTS (
    SELECT 1
    FROM events e
    WHERE e.legacy_source = 'planned_events' AND e.legacy_id = p.id
);
