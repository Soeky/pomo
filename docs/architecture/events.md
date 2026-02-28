# Events Domain Architecture

## Core Model

`events` is the canonical timeline model. It represents both planned and done work.

Core fields:
- `kind`: focus, break, task, class, exercise, meal, other
- `layer`: planned or done
- `status`: planned, in_progress, done, canceled, blocked
- `source`: manual, tracked, recurring, scheduler
- `domain` and `subtopic`: hierarchical topic path

## Compatibility Strategy

During migration:
- `sessions` and `planned_events` remain available for old flows.
- new writes can target `events`.
- migration backfills legacy rows into `events` with `legacy_source` and `legacy_id`.

Post-cutover (Task 12):
- primary runtime reads/writes use `events`.
- legacy sync triggers are removed by one-time v2 finalization (`FinalizeV2Cutover`).
- legacy ID assumptions are deprecated in calendar APIs; canonical IDs are `e-<id>`.

## Scheduler Inputs

Scheduler should consume:
- recurring rules
- workload targets
- schedule constraints
- dependency links

It produces planned `events` and stores run details in `schedule_runs` and `schedule_run_events`.
