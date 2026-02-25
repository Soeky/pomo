# 🍅 pomo

Minimal Pomodoro CLI with local SQLite storage, stats, and a built-in web UI.

## Features

- Focus and break sessions: `start`, `break`, `stop`, `status`
- Retroactive correction: `correct`
- Stats reports: `stat` with day/week/month/year/semester/date ranges
- Interactive delete flow: `delete`
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

## Commands

### Session Commands

```bash
pomo start [duration] [topic]
pomo break [duration]
pomo stop
pomo status
```

Duration format supports combined units such as `25m`, `1h`, `1h30m`, `45s`.

Examples:

```bash
pomo start 50m DeepWork
pomo start DeepWork            # topic only, uses default_focus
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
pomo stat 2026-02-25
pomo stat 2026-02
pomo stat 2026
pomo stat 2026-02-01 2026-02-25
```

`sem` starts at `semester_start` from config.

### Config

```bash
pomo set <default_focus|default_break|semester_start> <value>
```

Examples:

```bash
pomo set default_focus 30
pomo set default_break 7
pomo set semester_start 2026-02-10
```

### Database and Delete Tools

```bash
pomo db      # opens sqlite3 shell on pomo.db
pomo delete  # interactive session deletion prompt
```

`pomo db` requires `sqlite3` installed on your system.

### Web UI

```bash
pomo web start
pomo web stop
pomo web status
pomo web logs
pomo web hosts-check
```

Default web behavior:
- Runs as daemon by default (`--daemon=true`)
- Binds `127.0.0.1:3210` (or next free port up to `3299`)
- Opens browser automatically (`--open=true`)
- Uses `http://pomo:<port>` if hosts entry exists, always prints localhost fallback

Start flags:

```bash
pomo web start --host 127.0.0.1 --port 3210 --daemon --open
```

Web endpoints include dashboard (`/`), calendar (`/calendar`), sessions (`/sessions`), SQL console (`/sql`), and health (`/healthz`).

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
