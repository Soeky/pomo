-- Ensure recurring occurrence generation is idempotent per rule + start/end window.
CREATE UNIQUE INDEX IF NOT EXISTS idx_events_recurrence_occurrence_unique
ON events(recurrence_rule_id, start_time, end_time)
WHERE recurrence_rule_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_events_recurrence_rule_time
ON events(recurrence_rule_id, start_time);
