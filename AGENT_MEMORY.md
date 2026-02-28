# AGENT_MEMORY.md

## Mission
Transform `pomo` from a simple pomodoro tracker into a full time management application with unified planning + execution workflows across CLI, TUI, and web.

## Locked Product Decisions
- Topic syntax: `Domain::Subtopic`
- Missing subtopic defaults to `General`
- Scheduler v1: deterministic greedy + balanced-week spread
- Balanced-week config: explicit weekdays (not just count)
- Dependencies: hard-block dependents until prerequisites are done
- Break-credit: analytics-only (derived metrics), no source-row mutation
- Web stack: server-rendered templates + HTMX (no SPA)
- Event architecture: unified canonical events model
- TUI scope: planning/config/manage workflows are TUI; quick session ops stay plain CLI

## Canonical Definitions
- `planned`: scheduled/intended work
- `done`: completed/tracked work
- `blocked`: event that cannot proceed due to unmet dependencies
- `effective_focus`: focus time plus qualifying short breaks under configured threshold

## Scheduler Invariants
- Must not schedule outside configured day windows.
- Must respect active weekdays exactly.
- Must reserve meal breaks according to config.
- Must not place dependent events before prerequisite completion.
- Must be deterministic for identical inputs.

## CLI Conventions
- `pomo start [duration] <domain[::subtopic]>`
- If only domain is given, store as `Domain::General`.
- Keep legacy commands available during migration with explicit deprecation guidance.

## Compatibility Checklist
- Keep existing `sessions` and `planned_events` readable until explicit final cutover (`pomo upgrade` v2 finalization).
- Migrations must be idempotent.
- Existing user DBs must migrate without manual intervention.
- `go test ./...` must stay green per branch.

## Branch Progress Tracking
- Follow chronological branch order from `IMPLEMENTATION_PLAN.md`.
- Each feature branch must update this memory with:
  - schema changes
  - new/changed commands
  - metric definition changes
  - migration caveats

## Task 2 Decisions and Caveats (`feature/02-db-unified-events-migrations`)
- Added `events.timezone` with default `Local` for unified event rows.
- Legacy mapping uniqueness is enforced via `UNIQUE(legacy_source, legacy_id)` index to keep one canonical `events` row per legacy row.
- Backfill policy is idempotent insert+sync (safe to rerun): legacy rows from `sessions` and `planned_events` are inserted with `INSERT OR IGNORE`, then kept current via sync triggers.
- Compatibility adapters are DB triggers on `sessions`/`planned_events` (`INSERT`/`UPDATE`/`DELETE`) so legacy flows keep `events` synchronized until cutover.
- Backward-compat mapping for `planned_events` is preserved: migrated rows keep `domain=title` with `subtopic=General` until topic-hierarchy rollout.
- Migration caveat: if duplicate legacy mappings already exist in `events`, migration keeps the lowest `events.id` row and deletes duplicate legacy-mapped rows before creating the unique index.

## Task 3 Decisions and Caveats (DB Migration Reconciliation Hardening)
- Added follow-up migration `009_unified_events_reconcile_legacy_rows` to reconcile existing legacy-mapped `events` rows from `sessions`/`planned_events`.
- Reconciliation policy is rerun-safe: delete orphan legacy mappings, `INSERT OR IGNORE` missing mappings, then normalize/update existing mapped rows to legacy source-of-truth values.
- Invariant tightened for legacy-backed rows: `recurrence_rule_id`, `workload_target_id`, and `metadata_json` are cleared during reconciliation to prevent drift from legacy-compatible semantics.
- Migration caveat: rows in `events` with `legacy_source IN ('sessions', 'planned_events')` and no matching legacy parent row are treated as orphaned compatibility artifacts and removed.
- Ambiguity default used: user prompt named this work as "Task 3"; because requested deliverables were explicit migration/backfill/index tasks, execution followed those explicit deliverables.

## Task 3 Decisions and Caveats (Topic Hierarchy CLI/Web + Stats Completion)
- Escaped delimiter policy: `\::` is treated as a literal `::` inside a topic component; canonical output escapes both backslashes and literal delimiters in components.
- Parser invariant: `Domain` remains required; malformed combined topics with missing domain still fail validation.
- Web API compatibility policy:
  - Session endpoints accept combined `topic` or split `domain` + `subtopic`.
  - Calendar planned-event endpoints accept combined (`topic` or `title`) or split (`domain` + `subtopic`) representation.
  - Split representation is normalized to canonical `Domain::Subtopic`.
  - Legacy free-text planned-event `title` without delimiter remains preserved as-is for backward compatibility.
- Stats/reporting invariant:
  - Focus aggregation supports hierarchy rollups by domain and by subtopic.
  - Semester (`pomo stat sem`) reports include top-domain and top-subtopic sections.
  - Legacy flat topics continue to map to `Subtopic=General` in hierarchy aggregates.
- Ambiguity default used: because there was no prior explicit escape syntax rule, `\::` was selected as the parser escape convention and documented here.

## Task 4 Decisions and Caveats (Migration/Adapter Hardening for Unified Events)
- Added follow-up migration `010_unified_events_legacy_trigger_hardening` to reinforce legacy compatibility adapters.
- Legacy mutation invariant is now continuously enforced (not only on reconciliation reruns): when `sessions` or `planned_events` rows are inserted/updated, mapped `events` rows clear `recurrence_rule_id`, `workload_target_id`, and `metadata_json`.
- Migration remains rerun-safe: linkage-field cleanup is idempotent and trigger creation uses `IF NOT EXISTS`.
- Validation scope now explicitly includes unified scheduler schema/index presence plus replay/idempotency/parity checks in `internal/db/migrate_task4_test.go`.
- Ambiguity default used: although `feature/04` title focuses on recurring-event UX, execution followed the prompt’s explicit deliverables (DB migrations/backfill/indexing/parity/adapter stability) as the required Task 4 scope.

## Task 4 Decisions and Caveats (Single + Recurring Event Delivery)
- Recurrence RRULE support is intentionally scoped to `FREQ`, `INTERVAL`, and optional `BYDAY`/`BYMONTHDAY` keys with supported frequencies `DAILY`, `WEEKLY`, `MONTHLY`.
- Recurrence rule defaults:
  - `timezone` defaults to `Local` when omitted.
  - `kind` defaults to `task` when omitted.
  - Rules are created active by default in CLI/web.
- Monthly edge-date policy: when `BYMONTHDAY` exceeds a month’s last day, expansion clamps to the month end (for example, day `31` -> `30`/`29`/`28` depending month).
- Generated recurring occurrences persist into canonical `events` with `source=recurring`, `layer=planned`, `status=planned`, and populated `recurrence_rule_id` provenance.
- Idempotent generation invariant is enforced in schema via `011_recurring_events_occurrence_indexes`:
  - partial unique index `idx_events_recurrence_occurrence_unique(recurrence_rule_id, start_time, end_time) WHERE recurrence_rule_id IS NOT NULL`
  - lookup index `idx_events_recurrence_rule_time(recurrence_rule_id, start_time)`
- Calendar compatibility/API update:
  - mixed-source rendering includes `sessions`, `planned_events`, and canonical `events` (`e-<id>`).
  - calendar patch/delete endpoints now support `e-<id>` IDs in addition to legacy `s-<id>` and `p-<id>`.
- Web adapter safety default: recurrence/canonical event endpoints now return HTTP 500 (`database is not initialized`) when server runs with a mock store/no DB instead of panicking.
- Ambiguity default used: for short months in monthly recurrences, clamping-to-month-end was chosen over skipping that month.

## Task 5 Decisions and Caveats (Scheduler Prerequisite Migration + Reconciliation Hardening)
- Added migration `012_task5_scheduler_topic_backfill` to harden unified-event/scheduler indexing and refresh planned-event compatibility sync.
- Schema change:
  - `planned_events.domain` and `planned_events.subtopic` columns are introduced (`TEXT NOT NULL`, default `General`) to keep legacy planned rows topic-structured for scheduler-target reconciliation.
- Backfill/reconciliation policy:
  - During migration apply, every `planned_events.title` is parsed with the canonical topic parser (`\::` escape aware) and persisted into `planned_events.domain/subtopic`.
  - Migration then performs idempotent `INSERT OR IGNORE` + normalization update into `events` for `legacy_source='planned_events'`.
  - Legacy-backed scheduler linkage fields (`recurrence_rule_id`, `workload_target_id`, `metadata_json`) are cleared on reconciliation to preserve compatibility invariants.
- Adapter/runtime updates:
  - `store.CreatePlannedEvent` and `store.UpdatePlannedEvent` now derive and persist `domain/subtopic` from `title` to keep legacy writes and canonical `events` in parity.
  - `FinalizeV2Cutover` planned-event reconciliation now consumes `planned_events.domain/subtopic` (with fallback) instead of flattening to `domain=title`.
- New indexes introduced:
  - `idx_planned_events_topic`
  - `idx_events_workload_target_time` (partial)
  - `idx_events_source_status_time`
  - `idx_workload_targets_active_cadence_topic`
  - `idx_schedule_constraints_updated_at`
  - `idx_schedule_run_events_run_action`
- Ambiguity default used:
  - For legacy planned rows, `title` remains the compatibility source-of-truth and structured fields are derived from it during migration/reapply, rather than inferring from existing drifted `domain/subtopic` values.
- Scheduler v1 delivery decisions:
  - Constraints persistence key: `schedule_constraints.key = 'balanced_v1'` with JSON payload for weekdays/day windows/meal windows/max-hours/day/timezone.
  - Default `max_hours_per_day`: `8` when unset.
  - Target-semantics default:
    - if `target_occurrences > 0`, `target_seconds` is interpreted as per-occurrence duration (`Nx @ duration`).
    - otherwise `target_seconds` is interpreted as total duration per cadence unit.
  - Cadence unit counting:
    - `daily`: per active day in the window
    - `weekly`: per distinct ISO week that contains active days in the window
    - `monthly`: per distinct month that contains active days in the window
  - Deduction policy:
    - fixed commitments (`source != scheduler`) matching target topic reduce remaining target demand before generation.
    - existing scheduler events (`source=scheduler`, matching `workload_target_id`) are treated as already-satisfied demand when generation is not in replace mode.
  - Allocation policy:
    - deterministic greedy placement with round-robin day selection across active days.
    - scheduler never places events outside configured day windows, meal reservations, or day-cap limits.
    - minimum split chunk is 15 minutes; unresolved residual demand produces explicit `insufficient_capacity` diagnostics.
  - Apply-mode persistence (`pomo plan generate` without `--dry-run`):
    - writes generated rows into canonical `events` with `source=scheduler`, `layer=planned`, `status=planned`, `workload_target_id` linkage.
    - records run metadata in `schedule_runs` and created-event entries in `schedule_run_events`.

## Task 6 Decisions and Caveats (Dependencies + Blocking Enforcement)
- Added migration `013_task6_dependency_blocking_hardening`.
- Schema/index updates:
  - `events.blocked_reason` (`TEXT`, nullable)
  - `events.blocked_override` (`INTEGER NOT NULL DEFAULT 0`)
  - `idx_event_dependencies_required`
  - `idx_event_dependencies_dep_required`
  - `idx_events_status_override_time`
- Blocking reconciliation invariant:
  - planned events with unmet required dependencies are forced to `status=blocked` with a computed `blocked_reason`.
  - when required dependencies become satisfied, blocked events auto-transition to `status=planned` and clear `blocked_reason`.
  - `done`/`canceled` events are not auto-reblocked.
- Dependency graph policy:
  - cycles are rejected on insert/update via recursive path validation (`ErrDependencyCycle`).
  - delete dependency reconciles dependent closure to refresh blocked/unblocked state.
- Manual override policy:
  - override requires explicit admin confirmation (`--admin` on CLI, otherwise `ErrOverrideNotAllowed`).
  - override flips `blocked_override` and reconciliation respects this guard.
  - every override enable/disable writes an audit row to `audit_log` with actions:
    - `dependency_override_enable`
    - `dependency_override_disable`
- Scheduler enforcement policy:
  - apply-mode `plan generate` reconciles dependency blocking in the requested window after persistence.
  - dependency transitions are written into `schedule_run_events` using `block` or `update` actions with details JSON.
- Surface updates:
  - CLI:
    - new `pomo event dep add|list|delete|override`
    - `pomo event list` now prints `blocked_reason` for blocked events.
  - Web:
    - `/calendar/events` payload now includes `status` and `blocking_reason` for canonical events.
    - blocked canonical event titles include blocking context for immediate visibility in calendar UI.
- Migration replay caveat:
  - `013` is replay-safe; blocked fields are re-normalized and indexes are recreated idempotently.
- Ambiguity default used:
  - user prompt labeled this work as Task 6 but also listed prior migration/backfill deliverables; execution followed Task 6 acceptance criteria while preserving previously delivered unified-event backfill compatibility invariants.

## Task 7 Decisions and Caveats (Break Credit + Effective Focus Metrics)
- Effective-focus metrics are derived-only analytics:
  - no mutation of `sessions` or canonical `events` rows is performed while computing/reporting break credit.
- Break-credit rule (locked implementation):
  - a break is credited only when it is directly between adjacent focus sessions in timeline order (`start_time`, `id`) and both neighboring focus sessions resolve to the same domain.
  - eligibility is inclusive on threshold (`break_duration <= break_credit_threshold_minutes`).
- Domain resolution policy for break credit:
  - focus session topic is parsed using canonical `Domain::Subtopic` rules.
  - when parsing fails for legacy/free-text topics, the trimmed raw topic string is treated as domain (empty -> `General`).
- Config/runtime policy:
  - `break_credit_threshold_minutes` remains default `10`; metrics callers defensively fall back to `10` if runtime config is unset/invalid.
- Surface updates:
  - CLI `pomo stat` totals now expose raw focus, effective focus, credited breaks, and raw breaks.
  - web dashboard totals module now exposes raw focus, effective focus, credited breaks, and raw breaks.
- Ambiguity default used:
  - “between consecutive same-domain focus sessions” was implemented as immediate-neighbor focus checks (no skipping over intermediate non-focus rows).

## v2 Major Upgrade Decisions (Cutover + CLI Upgrade Command)
- `pomo upgrade` is the canonical in-CLI upgrade entrypoint (`pomo update` is an alias).
- Default `pomo upgrade` flow:
  - create a timestamped DB backup,
  - run schema migrations,
  - run one-time v2 cutover finalization,
  - run CLI self-update via `go install github.com/Soeky/pomo@<version>`.
- v2 cutover finalization is explicit and idempotent (`internal/db.FinalizeV2Cutover`):
  - reconciles/backfills legacy `sessions` and `planned_events` rows into `events`,
  - then drops legacy compatibility sync triggers,
  - then records completion in `app_meta` (`upgrade.v2_finalized=true`).
- Post-finalization invariant:
  - ongoing legacy-table-to-events compatibility sync is disabled by design.
  - this is the intentional major-version behavior change for dropping backward-compat sync.
- Ambiguity default used: self-update transport uses `go install` (module-tag based) rather than GitHub release binary download to keep implementation minimal and deterministic in current tooling.

## Task 8 Decisions and Caveats (Bubble Tea Management Suite)
- Root command TUI entrypoints:
  - `pomo event` now opens Bubble Tea event management.
  - `pomo plan` now opens Bubble Tea scheduler review/apply.
  - `pomo config` now opens Bubble Tea config wizard.
- Backward-compatibility policy preserved:
  - existing non-interactive subcommands remain available (`event add|list|recur|dep`, `plan target|constraint|generate|status`, `config list|get|set|describe`).
  - quick commands remain plain CLI (`start`, `break`, `stop`, `status`).
- Event-manager TUI scope:
  - supports single-event add/edit/delete, recurring-rule add/edit/delete, and dependency add/delete/override/list flows.
  - dependency override is always applied with explicit admin intent from TUI (`admin=true`, origin=`tui`) to preserve Task 6 policy.
- Config wizard persistence policy:
  - scheduler constraints are saved to `schedule_constraints` (`balanced_v1`) via scheduler service.
  - related defaults in `config.AppConfig` are also synchronized and written with `config.SaveConfig` (weekday/day window/meal windows + break threshold).
- Ambiguity default used:
  - for recurring-rule edit in TUI, leaving fields blank keeps current values; entering `-` for `until` clears rule end date.

## Task 9 Decisions and Caveats (Web UI Refresh + Runtime Mode)
- Runtime startup strategy chosen: `daemon + auto-sleep` (default).
  - daemon startup keeps warm health-check readiness gate before reporting success.
  - daemon process auto-sleeps after 15 minutes of non-health request inactivity.
- Web mode resolution policy (`pomo web start`):
  - precedence: `--mode` > compatibility `--daemon` > config `web_mode`.
  - valid modes remain `daemon` and `on_demand`.
  - backward compatibility preserved: `--daemon=false` maps to `on_demand`.
- Daemon lifecycle invariant:
  - hidden `web serve` now cleans stale runtime files (`web.pid`, `web.state.json`) on exit, including auto-sleep shutdown.
- Dashboard rendering policy update:
  - `/` now renders a lightweight shell and lazy-loads modules via HTMX route-level requests to `/dashboard/modules/<id>`.
  - module hydration windows are explicit: default last 7 days; `upcoming_schedule` uses next 7 days (`?window=upcoming`).
- Dashboard module surface update:
  - added `upcoming_schedule` card sourced from `planned_events` for schedule-first visibility.
- Asset optimization decision:
  - removed global Pico CSS dependency; templates rely on shared in-repo style system + route-scoped FullCalendar assets.
- Validation artifact note:
  - startup/memory comparisons are tracked in `docs/performance/task9_web_runtime.md` using isolated-home harness runs.

## Task 10 Decisions and Caveats (Help UX + Command IA)
- Help-surface IA now treats `config` as the canonical settings interface:
  - `pomo config get|set|list|describe` is documented in root/config help and README.
  - `pomo set` remains available as a compatibility alias and now always prints deprecation + migration guidance.
- New workflow help topic:
  - Added dedicated top-level `workflow` command with recommended day flow so both `pomo workflow` and `pomo help workflow` are supported.
- Command-group help enrichment:
  - `Long` help for root/event/plan groups now includes concrete examples for topic delimiter usage, recurring rules, scheduler flow, and dependency operations.
  - `event recur` and `event dep` subgroup help now include example-driven usage directly in command help output.
- Test coverage additions:
  - golden snapshots for key help outputs under `cmd/testdata/help/*.golden`.
  - integration-style CLI example validation tests execute documented examples in safe `--help` mode to ensure command/flag compatibility remains intact.

## Task 11 Decisions and Caveats (Dashboard Plan-vs-Actual + CLI Stats Extensions)
- Shared metrics engine:
  - Added unified plan-vs-actual metric computation in `internal/stats` and reused it for dashboard modules and CLI (`pomo stat adherence`, `pomo stat plan-vs-actual`) to enforce numerical parity.
- On-time adherence policy:
  - denominator: non-canceled planned rows in the selected window from:
    - legacy `planned_events`
    - canonical `events` rows where `layer='planned'` and `legacy_source!='planned_events'`
  - numerator: planned rows matched to same-domain focus sessions with start-time difference within tolerance.
  - matching is one-to-one and time-ordered per domain (a focus session can satisfy at most one planned row).
  - default tolerance is `±10 minutes` (`DefaultAdherenceToleranceMinutes`).
- Plan completion policy:
  - completion = `done / non-canceled planned` within window.
  - canceled planned rows are excluded from both completion and adherence denominators.
- Drift-by-domain policy:
  - planned minutes source: merged legacy `planned_events` + canonical non-legacy planned `events`, grouped by normalized domain.
  - actual minutes source: tracked focus sessions (`sessions.type='focus'`) aggregated by parsed topic domain.
  - drift is signed: `actual - planned` minutes.
- Weekly balance score policy:
  - computed per active day (day has planned or actual minutes): `1 - |actual-planned|/max(actual,planned)`.
  - weekly balance score is the average active-day score expressed as percent.
- Range defaults for new CLI surfaces:
  - `pomo stat adherence` and `pomo stat plan-vs-actual` default to current week when no range is provided.
  - explicit ranges reuse existing `pomo stat` date/timeframe parsing semantics.
- Ambiguity defaults used:
  - because Task 11 did not define tolerance or balance formula, default tolerance `10m` and daily alignment average were selected and documented above.

## Current Baseline
- Project currently has sessions + planned events + calendar + dashboard + SQL page.
- Config command IA now centers on `pomo config get|set|list|describe`; `pomo set` remains as deprecated compatibility alias.
