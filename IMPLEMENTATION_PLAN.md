# IMPLEMENTATION_PLAN.md — Pomo Migration to Full Time Management

## Summary
Migrate from session tracking to a unified time-management platform with:
- Hierarchical topics (domain + subtopic)
- Unified event model (planned + tracked + recurring + generated)
- Scheduler with balanced-week constraints and dependency handling
- Calendar parity across CLI and web
- TUI-first management flows
- Better help UX and stronger dashboard metrics

This plan is structured as sequential feature branches, each with clear agent ownership and acceptance gates.

## Execution Guardrails (Mandatory)
- Once a feature task is started, finish the full task scope (all listed acceptance criteria + required validations) before moving to the next feature, unless the user explicitly reprioritizes.
- A task may stay `in progress` only when there is an explicit blocker documented in that task section with:
  - blocker reason
  - concrete next action
  - owner and date
- `in progress` without a blocker is not allowed.
- Do not leave partial implementation carry-over between adjacent features without documenting exact remaining scope and reason in the same task section.
- Before marking a task `done`, run and record required validation gates for that branch (minimum: formatting + tests + vet where applicable).

## Assumptions and Locked Decisions
- Topic CLI format: delimiter syntax (`Domain::Subtopic`), subtopic defaults to `General`.
- Scheduler strategy v1: deterministic greedy placement with balanced-week constraints.
- Balanced week config: explicit weekdays + per-day time windows/caps.
- Dependency policy: hard block (dependent events cannot be scheduled/completed before prerequisites).
- Break-credit policy: analytics-only (no mutation of raw session rows).
- Web direction: server-rendered Go templates + HTMX (no SPA migration).
- Event model: unified canonical `events` domain model.
- TUI scope: planning/config/management flows become TUI; quick commands (`start`, `break`, `stop`, `status`) stay fast CLI.
- Codex memory file location: repository root (`AGENT_MEMORY.md`).

## Public API / Interface Changes (High-Level)
- New CLI syntax and commands:
  - `pomo start [duration] <domain[::subtopic]>`
  - New `pomo event ...` command group (single events, recurring, dependencies, list/edit/delete)
  - New `pomo plan ...` command group (targets, generate, review/apply, status)
  - New `pomo config ...` command group (replaces/extends unclear `set`)
- Web API endpoints extended:
  - `/calendar/events` upgraded to unified event CRUD with type/status/topic hierarchy fields
  - New endpoints for recurrence rules, targets, scheduler runs, dependency graph, config
- Core data interfaces:
  - `Event` (canonical), `RecurrenceRule`, `WorkloadTarget`, `ScheduleConstraint`, `EventDependency`, `ScheduleRun`, `MetricsSnapshot`
- Backward compatibility:
  - Existing `sessions`/`planned_events` migrated and mapped to unified `events` model
  - Legacy CLI commands preserved initially with deprecation help text

## Branch-by-Branch Chronological Plan

## 1) `feature/01-foundation-event-domain`
Goal: introduce canonical event architecture without breaking current behavior.

- Add architecture doc (`docs/architecture/events.md`) defining:
  - Event types: `focus`, `break`, `class`, `gym`, `task`, `meal`, `admin`
  - Event origin/source: `manual`, `tracked`, `recurring`, `scheduler`
  - Status lifecycle: `planned`, `in_progress`, `done`, `canceled`, `blocked`
- Create domain packages:
  - `internal/events` (types, validation, parsing for `domain::subtopic`)
  - `internal/scheduler` (interfaces only, no generation yet)
- Introduce compatibility adapters so current CLI/web continue functioning.
- Add migration scaffolding only (no destructive migration yet).

Agent split:
1. Schema/API design agent (types/contracts docs)
2. App integration agent (adapter wiring)
3. Test agent (contract + compatibility tests)

Acceptance:
- `go test ./...` green
- Existing commands/web unchanged in behavior

## 2) `feature/02-db-unified-events-migrations`
Goal: introduce new DB schema and backfill from old tables.

Status (2026-02-27): done
- Added hardening migration for unified events + scheduler-related indexes.
- Implemented safe/idempotent reconciliation backfill (`sessions`/`planned_events` -> `events`) via `INSERT OR IGNORE` plus sync triggers.
- Added compatibility sync triggers on legacy tables to keep `events` parity during transitional branches.
- Added migration/index/parity/idempotency tests.
- Added follow-up reconciliation sweep migration (`009_unified_events_reconcile_legacy_rows`) to repair drifted legacy mappings and prune orphan compatibility rows.

- New migrations:
  - `events` table (domain, subtopic, title, description, type, status, source, start/end/duration, timezone, metadata JSON)
  - `event_dependencies`
  - `recurrence_rules`
  - `workload_targets`
  - `schedule_constraints`
  - `schedule_runs` + `schedule_run_events`
- Backfill migration:
  - `sessions` -> `events` (`source=tracked`, type mapped)
  - `planned_events` -> `events` (`source=manual|scheduler`)
- Add indexes for time windows, status/type, domain/subtopic, dependency lookups.
- Keep old tables read-compatible until full cutover.

Agent split:
1. Migration authoring agent
2. Backfill verification agent
3. Query-performance/index agent

Acceptance:
- Migration idempotency tests
- Row-count reconciliation tests
- Time-range query parity tests vs previous implementation

## 3) `feature/03-topic-hierarchy-cli-web`
Goal: ship domain/subtopic tracking end-to-end.

Status (2026-02-28): done
- Added escaped-delimiter topic parsing (`\::`) with canonical escaping for delimiter/backslash safety.
- CLI topic flows (`start`/`correct`) now preserve hierarchical canonical output, including escaped edge cases.
- Web session and calendar create flows now support split `domain` + `subtopic` inputs while preserving combined input compatibility.
- Web API accepts combined or split topic representation (`topic` or `title`, and/or `domain` + `subtopic`) for session/calendar mutations.
- Stats now include hierarchy aggregation by domain and subtopic, and semester reports render top-domain/top-subtopic sections.
- Added parser, CLI/service, stats, and web integration tests to cover malformed delimiters, legacy topics, and split/combined web creation flows.

- Parsing rules:
  - `Math::Discrete Probability`
  - `Math` => subtopic `General`
  - escaped delimiter handling for edge cases
- CLI:
  - update `start`/`correct` to parse hierarchical topic
  - output formatting: show `Domain::Subtopic`
- Web:
  - calendar/session forms split into domain + subtopic inputs
  - API accepts combined or split representation
- Stats:
  - aggregate by domain and by subtopic
  - semester reports include top domains and top subtopics

Agent split:
1. Parser + CLI agent
2. Web form/API agent
3. Stats/reporting agent

Acceptance:
- parser table tests (single-word, multi-word, default subtopic, malformed delimiter)
- integration tests for CLI + web creation and listing
- regression tests for legacy topic strings

## 4) `feature/04-single-events-and-recurring-events`
Goal: support manual one-off events and recurring rules via CLI + web.

Status (2026-02-28): done
- Prerequisite unified-event migration layer is in place (`007` + `008` + `009`) with safe legacy backfill and reconciliation.
- Added Task 4 hardening migration (`010_unified_events_legacy_trigger_hardening`) to keep legacy-mapped rows aligned by clearing scheduler linkage fields on legacy mutations.
- Added recurring occurrence idempotency/index migration (`011_recurring_events_occurrence_indexes`) and extended migration coverage to assert recurrence occurrence indexes plus replay safety.
- Implemented recurrence rule CRUD + expansion engine (`DAILY`/`WEEKLY`/`MONTHLY`) with generated-event provenance (`recurrence_rule_id`) and safe idempotent window generation.
- Implemented full CLI recurring flows under `pomo event recur add|list|edit|delete|expand`.
- Calendar API/UI now supports recurrence rule management, mixed-source rendering (`sessions` + `planned_events` + canonical `events`), and canonical event update/delete via `e-<id>` IDs.
- Added acceptance tests:
  - recurrence expansion tests (DST-safe weekly expansion, weekly BYDAY patterns, monthly edge dates)
  - recurring + canonical single-event CRUD integration coverage
  - mixed-source calendar rendering coverage

- `pomo event add` (single event)
- `pomo event recur add` (daily/weekly/monthly with duration and optional domain/subtopic)
- recurrence expansion service:
  - generate occurrences in a date window
  - produce events with provenance to rule id
- Web:
  - recurrence UI in calendar side panel
  - recurring event management list/edit/delete

Agent split:
1. CLI command group agent
2. Recurrence engine agent
3. Web recurring UI agent

Acceptance:
- recurrence expansion tests (DST-safe, weekly patterns, month edge dates)
- CRUD integration tests for single and recurring events
- calendar rendering tests with mixed sources

## 5) `feature/05-workload-targets-and-balanced-scheduler-v1`
Goal: implement weekly/monthly/daily workload targets and balanced generation.

Status (2026-02-28): done
- Delivered prerequisite migration hardening (`012_task5_scheduler_topic_backfill`) with idempotent legacy planned-topic reconciliation and scheduler indexing updates.
- Implemented workload-target/constraint persistence and CLI surfaces:
  - `pomo plan target add|list|delete|set-active`
  - `pomo plan constraint show|set`
  - `pomo plan generate [--dry-run] [--replace]`
- Implemented scheduler v1 engine (deterministic balanced-week greedy allocator):
  - consumes existing fixed commitments first
  - respects explicit active weekdays, day window, meal windows, and max-hours/day cap
  - spreads generated workload across eligible days via deterministic round-robin day allocation
  - deducts fixed same-topic commitment time from target demand before generating scheduler rows
  - records run metadata in `schedule_runs` / `schedule_run_events` on apply mode
- Added scheduler acceptance tests:
  - deterministic snapshot stability
  - balanced distribution across selected weekdays
  - target satisfaction with fixed-event deductions
  - conflict/impossible diagnostics with missing-capacity reporting

- Add targets:
  - e.g. `Math 8h/week`, `Gym 4x/week @ 2h`
- Constraint model:
  - active weekdays (explicit)
  - day start/end
  - lunch/dinner windows and durations
  - max hours/day
- Scheduler v1:
  - deterministic greedy fill
  - spread remaining workload across configured weekdays
  - avoid stacking all workload into first days
  - consume fixed/recurring commitments first
- Linkage:
  - fixed lecture hours reduce remaining target hours automatically.

Agent split:
1. Constraint/config model agent
2. Scheduler algorithm agent
3. Integration + conflict-detection agent

Acceptance:
- deterministic scheduler snapshot tests
- balance tests (distribution across selected days)
- target satisfaction tests with fixed-event deductions
- conflict and impossible-plan diagnostics tests

## 6) `feature/06-dependencies-and-blocking`
Goal: enforce prerequisite chains in planning and execution.

Status (2026-02-28): done
- Added Task 6 hardening migration (`013_task6_dependency_blocking_hardening`) with replay-safe dependency/blocking indexes and blocked-field normalization.
- Implemented dependency graph service in `internal/events`:
  - add/list/delete dependency edges
  - cycle detection with recursive traversal validation
  - blocked/unblocked reconciliation for events and windows
  - admin-gated dependency override with audit log writes
- Scheduler apply runs now reconcile dependency status transitions in-window and persist `schedule_run_events` entries with `block`/`update` actions plus blocking diagnostics.
- CLI surfaces:
  - new `pomo event dep add|list|delete|override`
  - `event list` output now includes blocked reason for blocked events
- Web calendar payload now surfaces blocking state/reason for canonical events.
- Added acceptance coverage:
  - cycle detection tests
  - blocked/unblocked transition tests
  - scheduler generation/reconciliation tests with dependency constraints
  - migration replay/parity/index tests for Task 6 schema updates

- Dependency graph:
  - `tutorial` depends on `lecture`
- Enforcement:
  - scheduler marks dependent events `blocked` until prerequisite completion
  - CLI/web surfaces blocking reason
- Cycle detection in dependency graph.
- Manual override capability (admin flag + audit log).

Agent split:
1. Graph model + validation agent
2. Scheduler enforcement agent
3. UX + audit trail agent

Acceptance:
- cycle detection tests
- blocked/unblocked transition tests
- schedule generation tests with dependency constraints

## 7) `feature/07-break-credit-and-effective-time-metrics`
Goal: analytics logic for short-break inclusion within same domain.

Status (2026-02-28): done
- Implemented derived effective-focus metrics engine over ordered session rows with thresholded short-break credit.
- Added effective metric config support in runtime usage (`break_credit_threshold_minutes`, default `10`) and coverage for set/get/validation/default persistence.
- Integrated raw vs effective totals into CLI stats report output and dashboard totals module.
- Added acceptance coverage for threshold edge behavior, report raw-vs-effective output deltas, and no-mutation guarantees for `sessions`/`events` source rows.

- Implement “effective focus time” metric:
  - if break between consecutive same-domain focus sessions is <= threshold (default 10m), count break toward effective domain time
- Config:
  - `break_credit_threshold_minutes`
- Keep raw events unchanged; expose derived metrics in stats/dashboard.

Agent split:
1. Metrics engine agent
2. Config + CLI surface agent
3. Dashboard/stat integration agent

Acceptance:
- metrics unit tests for threshold edge cases
- report output tests comparing raw vs effective totals
- no mutation checks on source rows

## 8) `feature/08-tui-management-suite`
Goal: convert planning/config management flows to TUI (retain quick commands as plain CLI).

Status (2026-02-28): done
- Added Bubble Tea management suite under `internal/tui` and wired root command entrypoints:
  - `pomo event` launches event manager with single-event add/edit/delete, recurring-rule add/edit/delete, and dependency editor actions.
  - `pomo plan` launches scheduler review/apply screen (dry-run review and apply execution).
  - `pomo config` launches config wizard for weekday/day-window/meal-break constraints and break-threshold updates.
- Preserved non-TUI quick-session commands (`start`, `break`, `stop`, `status`) unchanged.
- Added acceptance-focused tests:
  - non-interactive model state transitions for event/scheduler/config models,
  - keyboard accessibility/navigation coverage (`arrows`, `j/k`, `tab`, `shift+tab`, `esc`, `q`/`ctrl+c`),
  - command startup/exit smoke tests for TUI entry commands.

- New TUIs (Bubble Tea):
  - Event manager (single/recurring add/edit/delete)
  - Scheduler review/apply screen
  - Config wizard (weekday constraints, day windows, meal breaks, thresholds)
  - Dependency editor
- Keep `start/break/stop/status` non-TUI for speed.

Agent split:
1. Shared TUI components agent
2. Planning/config TUI agent
3. Integration and usability test agent

Acceptance:
- non-interactive model tests (state transitions)
- smoke tests for command startup/exit paths
- accessibility/keyboard-navigation checks

## 9) `feature/09-web-ui-refresh-and-runtime-mode`
Goal: improve web UX while remaining lightweight.

- Design system refresh in templates/CSS:
  - clearer hierarchy, denser calendar controls, schedule-centric dashboard cards
- Keep HTMX + templates.
- Runtime mode improvements:
  - evaluate/startup strategy: daemon + auto-sleep OR on-demand start with warm health-check
  - expose `web mode` config and command help
- Optimize assets and route-level rendering.

Agent split:
1. UI/UX template agent
2. Runtime/daemon behavior agent
3. Performance profiling agent

Acceptance:
- web handler tests green
- startup latency benchmark comparison documented
- memory footprint check (before/after)

## 10) `feature/10-help-ux-and-command-information-architecture`
Goal: make CLI discoverable and self-explanatory.

- Replace ambiguous `pomo set` UX:
  - introduce `pomo config get|set|list|describe`
  - keep `set` as compatibility alias with warning
- Improve `Long` help for all command groups with examples:
  - topic delimiter examples
  - recurring rules
  - scheduler workflow
  - dependencies
- Add `pomo help workflow` with recommended daily flow.

Agent split:
1. Command IA/help text agent
2. Backward-compat alias agent
3. Docs/README agent

Acceptance:
- golden tests for help output
- command examples validated in integration tests
- README updated to new workflows

## 11) `feature/11-dashboard-plan-vs-actual`
Goal: planning accuracy and completion analytics.

- Dashboard modules:
  - On-time adherence (% events started within tolerance)
  - Plan completion (% planned done)
  - Drift (scheduled vs actual time per domain)
  - Weekly balance score
- CLI stats extensions:
  - `pomo stat adherence`
  - `pomo stat plan-vs-actual [range]`

Agent split:
1. Metrics query agent
2. Dashboard module agent
3. CLI reporting agent

Acceptance:
- metric correctness tests with seeded fixtures
- web module tests for empty/partial/full datasets
- cross-check tests between dashboard and CLI numbers

## 12) `feature/12-cutover-cleanup-and-deprecation`
Goal: complete migration and remove temporary compatibility layers.

Status (2026-02-28): in progress
- Added explicit major-upgrade cutover routine (`FinalizeV2Cutover`) that performs one-time legacy reconciliation and then disables legacy sync triggers.
- Added `pomo upgrade` / `pomo update` command path to run migrations, cutover finalization, and CLI self-update (`go install ...@version`).

- Switch primary reads/writes fully to unified `events`.
- Deprecate old direct-table assumptions.
- Add migration/version notes.
- Final performance and coverage gate pass.

Agent split:
1. Cutover/refactor agent
2. Migration safety agent
3. QA/release agent

Acceptance:
- full test suite + race + coverage gates pass
- migration from old user DB validated end-to-end
- release checklist and rollback notes completed

## Codex Memory File Plan (`AGENT_MEMORY.md`)
Create root file with:
- Product mission and non-goals
- Canonical glossary (event, target, recurrence, blocked, effective focus)
- Locked decisions (from above)
- CLI syntax canon (`domain::subtopic`, default subtopic `General`)
- Scheduler invariants (balanced weekdays, hard dependencies, deterministic output)
- Metrics definitions (raw vs effective)
- Testing gates required per branch
- Branch map and dependency graph
- “Do not break” compatibility checklist

Maintenance rule:
- Every merged feature branch updates:
  - schema changes
  - command changes
  - metric definitions
  - migration caveats
- Any non-done task must include an explicit blocker entry (reason + next action + owner/date); otherwise it must be closed in the same delivery cycle.

## Testing Matrix (Applies Across Branches)
- Unit:
  - parsers, recurrence expansion, scheduler allocation, dependency graph, metrics
- Integration:
  - CLI command behavior + DB writes
  - web handler CRUD and calendar responses
- Migration:
  - old DB fixture -> latest schema -> data parity checks
- Performance:
  - scheduler run time under realistic weekly load
  - web startup/daemon memory checks
- Regression:
  - legacy commands still operational until deprecation branch

## Tooling Suggestions
- Keep: `Bubble Tea` for TUI, `Cobra` for CLI.
- Add:
  - `github.com/charmbracelet/huh` for form-heavy TUI wizards
  - `github.com/teambition/rrule-go` for robust recurring rule handling
  - `github.com/olekukonko/tablewriter` (or similar) for readable CLI tabular plan/review output
- Optional later:
  - SQLite query plan checks in CI for scheduler/dashboard heavy queries.

## Open Clarification (Non-blocking, default chosen)
- Timezone policy for schedule generation/reporting is not explicitly defined; default to local system timezone with UTC storage normalization where possible.
