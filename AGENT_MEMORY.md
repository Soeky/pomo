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

## Current Baseline
- Project currently has sessions + planned events + calendar + dashboard + SQL page.
- `pomo set` exists but is unclear; target is `pomo config get|set|list|describe`.
