# PersonalCRM — Agent Instructions

This file is the root Antigravity project rule for **PersonalCRM** (`pcrm` — Personal Projects, Ideas, Hobbies & Routines CRM). It is always loaded when working in this repository. Keep it concise and under 24 KB; put detailed specifications in `docs/`.

## Project Summary

**PersonalCRM** is a lightweight, local-first, cross-platform **TUI** application written in **Go**, distributed as a portable single executable (`pcrm.exe` on Windows, `pcrm` on Debian Linux and macOS).

- **Primary target (v1):** Windows (portable single `.exe`, zero CGO required for Windows/Linux builds)
- **Secondary targets:** Debian Linux, macOS
- **Runtime model:** Single binary supporting **TUI client**, **background system tray daemon** (autostart, scheduler, OS notifications), and **CLI quick-capture** (`pcrm add ...`).
- **Core pillars:** **Projects**, **Ideas Tracker** (serves as Inbox + research hub), **Hobbies**, and **Routines**, backed by standalone reusable **Notes**, **Resources/Links**, **Progress Logs**, and a **Weekly Refinement** workflow.
- **Extensibility:** Built around a **pluggable module architecture** so future capabilities (e.g., **People**, **Reports**, custom views/integrations) can be added and toggled cleanly without rewriting core storage or TUI code.

## Canonical Documentation

Always read the relevant docs before making architectural or implementation decisions:

- [docs/PROJECT_CONTEXT.md](docs/PROJECT_CONTEXT.md) — Confirmed product vision, domain model, workflows, and UX requirements.
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — Technical architecture, Go stack, SQLite schema, pluggable module system, tray/daemon lifecycle, and package layout.
- [docs/STATUS.md](docs/STATUS.md) — Current project phase, Architecture Decision Records (ADRs), and milestone tracker.
- [docs/RUST_VS_GO.md](docs/RUST_VS_GO.md) — Tech stack trade-off analysis (Rust vs. Go) for reference.

## Engineering & Working Guidelines

1. **Follow ADRs:** Do not deviate from locked decisions in `docs/STATUS.md` without recording a superseding ADR.
2. **Preserve pluggability & polymorphic data modeling:** Notes, Resources/Links, Tags, and Progress Logs attach polymorphically via `(entity_type, entity_id)` so future plugin entities (e.g., `person`) work automatically without core schema changes.
3. **Cross-platform by design:** Isolate all OS-specific code (system tray, notifications, autostart, terminal launching, paths, process detachment) inside `internal/platform` behind Go interfaces using build tags (`_windows.go`, `_linux.go`, `_darwin.go`).
4. **Zero-CGO core:** Use pure-Go SQLite (`modernc.org/sqlite`) and avoid CGO in core packages so Windows portable `.exe` and cross-builds remain simple and reliable.
5. **Local-first & data safety:** Use SQLite in WAL mode with foreign keys enabled, forward-only versioned migrations, and never introduce destructive schema changes without backup/migration safety.
6. **Separation of concerns:** Keep `internal/domain` & `internal/storage` strictly independent from `internal/tui`, `internal/daemon`, and `internal/cli`.
7. **Keep docs synchronized:** Update `docs/STATUS.md` when milestones complete or decisions evolve.
8. **Verify changes:** Run `go test ./...` and `go vet ./...` (and `go build ./...`) before marking implementation work complete.

## Repository Conventions

- `AGENTS.md` — Always-on Antigravity instructions (this file).
- `docs/` — Product context, architecture blueprint, status, and ADRs.
- `cmd/pcrm/` — Single binary entrypoint (`tui`, `daemon`, `add`, `export`, `backup`, etc.).
- `internal/domain/` — Pure domain models, validation, recurrence & refinement rules.
- `internal/storage/` — SQLite repository, migrations, export/backup.
- `internal/ext/` — Pluggable extension/module registry, event bus, and optional modules.
- `internal/tui/` — Bubble Tea TUI views, components, and `$EDITOR` integration.
- `internal/daemon/` — Background tray host, refinement/reminder scheduler, single-instance IPC.
- `internal/platform/` — OS-specific implementations behind shared interfaces.
