UPDATE sessions
SET created_at = COALESCE(created_at, start_time),
    updated_at = COALESCE(updated_at, COALESCE(end_time, start_time));
