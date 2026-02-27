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
- Keep existing `sessions` and `planned_events` readable until final cutover.
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

## Current Baseline
- Project currently has sessions + planned events + calendar + dashboard + SQL page.
- `pomo set` exists but is unclear; target is `pomo config get|set|list|describe`.
