# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test

```bash
# Enter dev shell (required for all commands)
nix develop

# Build
go build ./...

# Run all tests
go test ./...

# Run a single package's tests
go test ./internal/store/ -v

# Run a single test
go test ./internal/tracker/ -run TestMeetingSplitsSession -v

# Build the Nix package
nix build .

# Lint
golangci-lint run
```

Note: `git add` new files before `nix develop` or `nix build` — Nix flakes only see git-tracked files.

When changing Go dependencies, the `vendorHash` in `flake.nix` must be updated (build with a dummy hash, copy the correct one from the error).

## Architecture

Backbeat is a **daemon + CLI** that tracks active desktop time and syncs worklogs to Jira Tempo.

### Event pipeline

Three monitors run as goroutines, emitting `monitor.Event` into a single shared channel (`cap=32`). The `tracker.Tracker` is the sole consumer — it runs a state machine (Idle/Active/Sleeping) and writes sessions to SQLite via `store.Store`.

```
ActivityMonitor (D-Bus: Mutter IdleMonitor, polls GetIdletime) ──┐
SleepMonitor    (D-Bus: logind PrepareForSleep signal)           ├─ chan Event ─→ Tracker FSM ─→ Store (SQLite)
MeetingMonitor  (shells out to pw-dump, parses JSON)             ┘
```

Meeting events are orthogonal to the Idle/Active/Sleeping FSM — they split the current session into meeting/non-meeting segments rather than changing state.

### CLI ↔ Daemon communication

CLI commands (`status`, `track`, `sync`, `stop`) send JSON over a Unix socket at `$XDG_RUNTIME_DIR/backbeat.sock`. The daemon implements `ipc.Handler`. The `log` command is the exception — it reads SQLite directly (WAL mode allows concurrent reads).

### Tempo sync flow

`HandleSync()` in daemon.go: queries unsynced sessions → aggregates by (issue_key, date) → resolves issue keys to numeric IDs via Jira REST (cached in `issue_cache` table) → POSTs to Tempo API v4 `/4/worklogs` → marks sessions as synced.

### Key types

- `monitor.Event` — all monitor output flows through this (event.go)
- `store.Session` — a tracked time segment with optional issue key and meeting flag
- `tracker.State` — FSM states: `StateIdle`, `StateActive`, `StateSleeping`
- `ipc.Handler` — interface the daemon implements for CLI commands
- `config.Duration` — wraps `time.Duration` for TOML marshal/unmarshal

### Data paths

- Config: `$XDG_CONFIG_HOME/backbeat/config.toml` (default `~/.config/backbeat/config.toml`)
- Database: `$XDG_DATA_HOME/backbeat/backbeat.db` (default `~/.local/share/backbeat/backbeat.db`)
- Socket: `$XDG_RUNTIME_DIR/backbeat.sock`

## Testing patterns

- Store tests use in-memory SQLite (`:memory:` DSN) — no filesystem or cleanup needed
- Meeting monitor tests use a `CommandRunner` interface to mock `pw-dump` output
- Tracker tests send events to a channel and use `time.Sleep(50ms)` for goroutine processing
- Tests requiring D-Bus (activity, sleep monitors) are manual-only — no mocking for those

## NixOS integration

`flake.nix` exports `homeManagerModules.default` and `overlays.default`. Users enable it with `services.backbeat.enable = true` in Home Manager config after adding the flake as an input and applying the overlay.
