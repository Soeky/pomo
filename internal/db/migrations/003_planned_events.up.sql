CREATE TABLE IF NOT EXISTS planned_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    description TEXT,
    start_time DATETIME NOT NULL,
    end_time DATETIME NOT NULL,
    status TEXT NOT NULL DEFAULT 'planned' CHECK(status IN ('planned','done','canceled')),
    source TEXT NOT NULL DEFAULT 'manual' CHECK(source IN ('manual','scheduler')),
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_planned_events_time ON planned_events(start_time, end_time);
