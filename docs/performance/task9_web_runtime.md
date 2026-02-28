# Task 9 Runtime Validation (Web Startup + Memory)

Date: 2026-02-28

## Chosen Runtime Strategy
- Default mode: `daemon` with auto-sleep after 15 minutes of inactivity.
- Alternative mode: `on_demand` foreground startup with warm health-check before auto-open.

## Measurement Method
- Built local binaries before and after Task 9 changes.
- Ran `pomo web start --host 127.0.0.1 --port 325x --open=false --mode daemon` in an isolated temporary `HOME` for each run.
- Captured startup latency as wall-clock time of `web start` command completion.
- Captured daemon memory as RSS (`ps -o rss`) from PID stored in `~/.local/share/pomo/web.state.json`.
- Runs per sample: 6.

## Results
| Metric | Before Task 9 | After Task 9 | Delta |
|---|---:|---:|---:|
| Avg startup latency | 319 ms | 342 ms | +23 ms |
| Avg daemon RSS | 21008 KB | 21101 KB | +93 KB |

## Notes
- Startup increase is small and expected from mode resolution + daemon auto-sleep instrumentation.
- Memory delta is within normal process variance for this workload profile.
- Route-level dashboard rendering reduced initial dashboard DB work (shell-first HTMX hydration), which is not directly reflected in daemon bootstrap timing above.
