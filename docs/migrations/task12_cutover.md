# Task 12 Cutover Notes

Date: 2026-02-28

## Migration / Version Notes
- Cutover target: v2 canonical event model (`events`) as the primary runtime store.
- Legacy compatibility sync (`sessions`/`planned_events` -> `events`) remains only as a migration bridge before finalization.
- One-time finalization command path: `pomo upgrade` (or `pomo update`) executes:
  1. DB backup
  2. migrations
  3. `FinalizeV2Cutover` (reconciliation + compatibility trigger removal)
  4. optional CLI self-update (`go install github.com/Soeky/pomo@<version>`)
- Post-finalization behavior:
  - runtime reads/writes use canonical `events` paths,
  - legacy `sessions`/`planned_events` rows are no longer updated by ongoing app writes,
  - legacy calendar IDs (`s-<id>`, `p-<id>`) are deprecated and only accepted as compatibility lookups when legacy mappings exist.

## Release Checklist
- Run and verify:
  - `gofmt` on changed Go files
  - `go test ./...`
  - `go test -race ./...`
  - `make test-cover`
  - `go vet ./...`
- Validate migration safety on legacy fixture:
  - old schema fixture migrates to latest,
  - finalization keeps legacy->event parity,
  - post-cutover writes create canonical non-legacy `events` rows,
  - legacy source tables remain unchanged after post-cutover writes.
- Manual smoke checks:
  - `pomo start/break/stop/status` behavior remains correct,
  - `pomo stat`, `pomo stat adherence`, `pomo stat plan-vs-actual` report sane values,
  - web `/calendar` CRUD works with canonical `e-<id>` IDs,
  - web `/sessions` and dashboard modules render expected tracked/planned data.

## Rollback Notes
- Immediate rollback path:
  1. stop running `pomo` processes,
  2. restore the backup created by `pomo upgrade` (`pomo.db.bak.<timestamp>`),
  3. restart with the previous binary version.
- If finalization already ran (`app_meta.upgrade.v2_finalized=true`):
  - restored backup is the safest path to regain legacy trigger behavior,
  - in-place rollback is not recommended because triggers were intentionally removed and new writes are canonical-only.
- Data caveat:
  - post-cutover canonical-only writes are not mirrored into legacy tables; restoring an old backup discards those newer canonical writes unless separately exported.
