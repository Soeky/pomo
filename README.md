# 🍅 pomo

Minimal Pomodoro CLI with local SQLite storage, stats, and a built-in web UI.

## Features

- Focus and break sessions: `start`, `break`, `stop`, `status`
- Retroactive correction: `correct`
- Stats reports: `stat` with day/week/month/year/semester/date ranges (raw + effective focus totals)
- Interactive delete flow: `delete`
- Unified event CRUD (single events): `event add|list`
- Workload targets + balanced scheduler: `plan target|constraint|generate`
- One-command major upgrade flow: `upgrade` / `update`
- Plan progress summary: `plan status`
- Structured config management: `config list|get|set|describe`
- Guided daily workflow help: `help workflow`
- Direct DB shell: `db` (via `sqlite3`)
- Built-in web server: `web start|stop|status|logs|hosts-check`
- Shell completion generation: `completion` (bash/zsh/fish/powershell)
- Automatic DB migrations on startup

## Install

```bash
go install github.com/Soeky/pomo@latest
```

Or build locally:

```bash
git clone https://github.com/Soeky/pomo.git
cd pomo
make build
```

## Quick Start

```bash
pomo start
pomo status
pomo break 10m
pomo stop
```

By default:
- `start` uses `default_focus` minutes and topic `General`
- `break` uses `default_break` minutes
- Starting a new session stops any currently running session first

### Help & Workflow

```bash
pomo --help
pomo help workflow
pomo workflow
```

`pomo help workflow` provides the recommended daily flow:
1. review plan status
2. tune targets and constraints
3. run scheduler dry-run/apply
4. execute sessions (`start` / `break`)
5. review day/plan metrics

## Commands

### Session Commands

```bash
pomo start [duration] [domain::subtopic]
pomo break [duration]
pomo stop
pomo status
```

Duration format supports combined units such as `25m`, `1h`, `1h30m`, `45s`.

Examples:

```bash
pomo start 50m Math::DiscreteProbability
pomo start "Applied Math::Numerical Analysis"
pomo start Math                # stored as Math::General, uses default_focus
pomo start "Math\\::History::Week 1" # escaped delimiter => domain Math::History, subtopic Week 1
pomo break 7m
```

### Correct Missed Start/Break

```bash
pomo correct [start|break] [time-into-past] [topic]
```

Example:

```bash
pomo correct start 15m ProjectX
```

This starts `ProjectX` at `now-15m` and closes the previous running session at that same timestamp.

### Stats

```bash
pomo stat
pomo stat day|week|month|year|all|sem
pomo stat adherence
pomo stat plan-vs-actual
pomo stat plan-vs-actual 2026-02-01 2026-02-25
pomo stat 2026-02-25
pomo stat 2026-02
pomo stat 2026
pomo stat 2026-02-01 2026-02-25
```

`sem` starts at `semester_start` from config.

### Config

```bash
pomo config              # launches config wizard TUI
pomo config list
pomo config get <key>
pomo config set <key> <value>
pomo config describe [key]
```

`pomo set` is still available as a compatibility alias for `pomo config set`, and now prints deprecation guidance.

Effective-focus break credit is controlled by `break_credit_threshold_minutes` (default `10`):

```bash
pomo config set break_credit_threshold_minutes 10
```

### Database and Delete Tools

```bash
pomo db      # opens sqlite3 shell on pomo.db
pomo delete  # interactive session deletion prompt
```

`pomo db` requires `sqlite3` installed on your system.

### Unified Event Commands

```bash
pomo event               # launches event manager TUI
pomo event add --title "Math study block" --start 2026-03-01T10:00 --end 2026-03-01T11:30 --domain Math --subtopic "Discrete Probability"
pomo event list --from 2026-03-01T00:00 --to 2026-03-08T00:00
pomo event recur add --title "Weekly Review" --start 2026-03-02T09:00 --duration 1h --freq weekly --byday MO,WE --domain Planning --subtopic General
pomo event recur list --active-only
pomo event recur edit 1 --title "Deep Review" --interval 2
pomo event recur expand --from 2026-03-01T00:00 --to 2026-03-31T23:59
pomo event recur delete 1
pomo event dep add 42 41 --required
pomo event dep list 42
pomo event dep override 42 --admin --reason "manual validation done offline"
```

### Plan & Scheduler

```bash
pomo plan                # launches scheduler review/apply TUI
pomo plan status --from 2026-03-01T00:00 --to 2026-03-08T00:00
pomo plan target add --domain Math --subtopic Discrete --cadence weekly --hours 8
pomo plan target add --title "Gym sessions" --domain Gym --subtopic General --cadence weekly --occurrences 4 --duration 2h
pomo plan target list --active-only
pomo plan constraint show
pomo plan constraint set --weekdays mon,tue,wed,thu,fri --day-start 08:00 --day-end 22:00 --lunch-start 12:30 --lunch-duration 60 --dinner-start 19:00 --dinner-duration 60 --max-hours-day 8 --timezone Local
pomo plan generate --from 2026-03-01T00:00 --to 2026-03-08T00:00 --replace
pomo plan generate --from 2026-03-01T00:00 --to 2026-03-08T00:00 --dry-run
```

The quick session commands `start`, `break`, `stop`, and `status` remain non-TUI for speed.

### Upgrade / Update

```bash
pomo upgrade
pomo update                    # alias
pomo upgrade --version v2.0.0
pomo upgrade --skip-self       # run db migration/finalization only
```

Default `pomo upgrade` behavior:
- creates a timestamped DB backup next to `pomo.db`
- applies DB migrations
- runs one-time v2 cutover finalization (legacy backfill reconciliation + disables legacy sync triggers)
- self-updates CLI using `go install github.com/Soeky/pomo@<version>`

Cutover notes:
- runtime reads/writes are canonical `events` after Task 12 cutover.
- legacy compatibility IDs in calendar APIs (`s-<id>`, `p-<id>`) are deprecated; use `e-<id>`.
- release checklist + rollback notes: `docs/migrations/task12_cutover.md`.

For self-update, `go` must be available in your `PATH`.

### Web UI

```bash
pomo web start
pomo web stop
pomo web status
pomo web logs
pomo web hosts-check
```

Web runtime modes:
- `daemon` (default): starts background web process with auto-sleep after 15 minutes of inactivity
- `on_demand`: runs in foreground and opens browser only after warm health-check passes

Mode resolution order:
1. `--mode` flag
2. compatibility `--daemon` flag
3. `web_mode` config value

Set default mode:

```bash
pomo config set web_mode daemon
pomo config set web_mode on_demand
```

Start flags:

```bash
pomo web start --host 127.0.0.1 --port 3210 --mode daemon --open=false
pomo web start --mode on_demand
pomo web start --daemon=false      # compatibility alias for --mode on_demand
```

Web endpoints include dashboard (`/`), calendar (`/calendar`), sessions (`/sessions`), SQL console (`/sql`), and health (`/healthz`).

CLI-equivalent web workspaces:
- `/events`: add/list/delete events, manage recurrence rules, run recurrence expansion.
- `/dependencies`: add/list/delete dependencies and toggle blocked overrides.
- `/planner`: status, generate (dry-run/apply), target CRUD, constraints update.
- `/reports`: `stat`, `adherence`, and `plan-vs-actual` report rendering.
- `/config`: list/get/set/describe config keys.
- `/delete`: recent sessions + bulk delete.
- `/workflow`: guided daily command flow.

Quick web flow:
1. Start web: `pomo web start`
2. Open `http://127.0.0.1:3210/events` to add events/recurrence.
3. Open `http://127.0.0.1:3210/dependencies` to link prerequisite events.
4. Open `http://127.0.0.1:3210/planner` to set targets/constraints and generate schedule.

## Shell Completion

Generate completion directly:

```bash
pomo completion bash
pomo completion zsh
pomo completion fish
pomo completion powershell
```

Or use the helper script:

```bash
./scripts/install_completion.sh
```

## File Locations

- Config: `~/.config/pomo/config.json`
- Database: `~/.local/share/pomo/pomo.db`
- Web daemon state: `~/.local/share/pomo/web.state.json`
- Web daemon PID: `~/.local/share/pomo/web.pid`
- Web daemon logs: `~/.local/share/pomo/web.log`

## Development

```bash
make test
make test-cover
make test-race
make vet
```
