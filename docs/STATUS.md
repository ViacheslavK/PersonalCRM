# PersonalCRM — Project Status & Decisions

> **Current Phase:** `02 - Architecture Locked / Ready for Implementation`  
> **Last Updated:** 2026-11-24

---

## 1. Current State

- **Completed:** Discovery finalized, all high-level product & architectural questions (`Q1`–`Q8`) resolved, ADRs recorded, and technical blueprint documented in [docs/ARCHITECTURE.md](ARCHITECTURE.md).
- **Next Step:** Begin **Phase 2 — Go Project Bootstrap, Core Domain & SQLite Storage** (once the user gives the go-ahead on the blueprint).

---

## 2. Confirmed Requirements

- [x] Cross-platform TUI application (Windows portable single `.exe` first; Debian Linux and macOS via platform interfaces).
- [x] Single binary (`pcrm`) with background system tray daemon + terminal TUI client + CLI quick capture.
- [x] Autostart support (in-app toggle) and native OS notifications.
- [x] Four core pillars: **Projects**, **Ideas Tracker** (doubles as Inbox), **Hobbies** (dated progress logs, active/on-hold/rejected, no end target), and **Routines** (daily/weekly/monthly/every-N-days/specific-weekdays simple resetting checklist).
- [x] Standalone reusable **Notes** (edited via external `$EDITOR`) and **Resources/Links** with many-to-many linking across entities.
- [x] **Weekly Refinement** over Projects (excluding `finished`/`rejected`/`on_hold`), Ideas, and Routines — catch-up on missed schedules, prominent Dashboard banner (non-blocking), plus opt-in Routine and Task deadline notifications.
- [x] **Pluggable module architecture** so future features (e.g., **People**, **Reports**) can be added and enabled cleanly without rewriting core.

---

## 3. Architecture Decision Records (ADRs)

| ADR | Title | Decision | Date |
| :--- | :--- | :--- | :--- |
| **ADR-001** | Tray + TUI Window Model | Single binary operating as a background system tray daemon + terminal TUI client (`1A`). Closing the TUI leaves the tray daemon alive until explicit Quit. | 2026-11-24 |
| **ADR-002** | Programming Language & TUI Stack | **Go** with Bubble Tea / Lip Gloss / Bubbles (`2B`). See [RUST_VS_GO.md](RUST_VS_GO.md) for rationale. | 2026-11-24 |
| **ADR-003** | Storage Engine & Portability | Local **SQLite** via pure-Go `modernc.org/sqlite` (zero CGO) in WAL mode as single source of truth, plus built-in Markdown/JSON export and backup (`3A`). | 2026-11-24 |
| **ADR-004** | Domain Model & Polymorphic Linking | Notes and Resources/Links are standalone reusable entities linked many-to-many via polymorphic `entity_links`; Progress Logs and Tags are also polymorphic. Inbox is merged into Ideas Tracker (`inbox` status). | 2026-11-24 |
| **ADR-005** | Pluggable Extension Architecture | Modular `internal/ext` registry with config-toggled modules, namespaced migrations, event bus, and TUI/CLI/Refinement hooks — deferring `People` and `Reports` to future modules without core schema changes. | 2026-11-24 |
| **ADR-006** | Routines Recurrence & Reset | Support `daily`, `weekly`, `monthly`, `every_n_days`, and `weekdays` schedules using deterministic cycle keys so routines act as a simple resetting checklist without history bloat. | 2026-11-24 |
| **ADR-007** | Weekly Refinement & Notifications | Weekly schedule with startup catch-up covering active/waiting Projects (+ open tasks), Ideas, and Routines; opens normal Dashboard with a prominent Refinement banner. Opt-in reminders for due Routines and task deadlines. | 2026-11-24 |
| **ADR-008** | Editing, Quick Capture & Windows Packaging | External `$EDITOR` for longer notes/descriptions; CLI quick-capture (`pcrm add ...`) + tray menu actions; portable single `pcrm.exe` on Windows with zero-flash daemon spawning. | 2026-11-24 |

---

## 4. Milestone Roadmap

- [x] **Phase 1 — Discovery & Architecture:** Requirements synthesized, ADRs locked, SQLite schema & pluggable architecture designed.
- [ ] **Phase 2 — Core Engine & Storage:** Go module bootstrap, config/portable-mode detection, domain models, recurrence & refinement math, SQLite migrations & repositories, unit tests.
- [ ] **Phase 3 — Extension Registry & CLI Quick Capture:** `internal/ext` module registry + event bus, `pcrm add`, `pcrm export`, `pcrm backup`.
- [ ] **Phase 4 — TUI Application:** Bubble Tea views for Dashboard, Projects/Tasks, Ideas (+ Promote flow), Hobbies, Routines, Notes/Resources (+ `$EDITOR`), Progress Logs, and guided Weekly Refinement.
- [ ] **Phase 5 — Windows Tray Daemon, Notifications & Autostart:** Windows `internal/platform` backend, background scheduler, IPC, and portable `pcrm.exe` build.
- [ ] **Phase 6 — Cross-Platform & First Extensions:** Debian Linux & macOS backends, plus initial optional plugins (e.g., `people`, `reports`).
