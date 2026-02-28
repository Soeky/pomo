-- Keep legacy-backed compatibility rows free of scheduler-only linkage fields.
-- Safe to rerun.
UPDATE events
SET recurrence_rule_id = NULL,
    workload_target_id = NULL,
    metadata_json = NULL
WHERE legacy_source IN ('sessions', 'planned_events');

CREATE TRIGGER IF NOT EXISTS trg_sessions_to_events_insert_clear_linkage
AFTER INSERT ON sessions
BEGIN
    UPDATE events
    SET recurrence_rule_id = NULL,
        workload_target_id = NULL,
        metadata_json = NULL
    WHERE legacy_source = 'sessions'
      AND legacy_id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS trg_sessions_to_events_update_clear_linkage
AFTER UPDATE ON sessions
BEGIN
    UPDATE events
    SET recurrence_rule_id = NULL,
        workload_target_id = NULL,
        metadata_json = NULL
    WHERE legacy_source = 'sessions'
      AND legacy_id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS trg_planned_events_to_events_insert_clear_linkage
AFTER INSERT ON planned_events
BEGIN
    UPDATE events
    SET recurrence_rule_id = NULL,
        workload_target_id = NULL,
        metadata_json = NULL
    WHERE legacy_source = 'planned_events'
      AND legacy_id = NEW.id;
END;

CREATE TRIGGER IF NOT EXISTS trg_planned_events_to_events_update_clear_linkage
AFTER UPDATE ON planned_events
BEGIN
    UPDATE events
    SET recurrence_rule_id = NULL,
        workload_target_id = NULL,
        metadata_json = NULL
    WHERE legacy_source = 'planned_events'
      AND legacy_id = NEW.id;
END;
