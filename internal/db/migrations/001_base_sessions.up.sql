CREATE TABLE IF NOT EXISTS sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL CHECK(type IN ('focus', 'break')),
    topic TEXT,
    start_time DATETIME NOT NULL,
    end_time DATETIME,
    duration INTEGER
);
CREATE INDEX IF NOT EXISTS idx_start_time ON sessions(start_time);
