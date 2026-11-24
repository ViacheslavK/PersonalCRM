# PersonalCRM — Automated Test Plan & Inventory

> **Status:** Active (Updated through Phase 3)  
> **Last Updated:** 2026-11-24

---

## 1. How to Run Verification

Run from the repository root:

```powershell
# Static analysis
go vet ./...

# Full automated test suite (verbose)
go test -v ./...

# Zero-CGO portable Windows binary build check
$env:CGO_ENABLED = "0"
go build -o pcrm.exe ./cmd/pcrm
```

---

## 2. Current Automated Test Inventory (19 Test Functions)

### 2.1 Configuration & Platform Paths (`internal/config`)
File: [internal/config/config_test.go](../internal/config/config_test.go)

| Test Function | Type | What It Verifies |
| :--- | :--- | :--- |
| `TestConfigDefaultLoadSave` | Unit / FS | Missing config file loads defaults (`sunday 18:00`, refinement notifications on, routine/task reminders opt-in); atomic TOML save & reload preserves editor, autostart, schedule, notifications, and plugin toggle map. |
| `TestConfigValidation` | Unit | Rejects invalid weekday names, malformed `HH:MM` clock strings, and `snooze_hours < 1`. |
| `TestResolvePathsPrecedence` | Unit / FS | Explicit `--data-dir` override takes precedence over `PCRM_DATA_DIR`, resolves `pcrm.db`, and creates required directories via `platform.EnsureDirs`. |

### 2.2 Pure Domain Logic & Schedulers (`internal/domain`)
File: [internal/domain/domain_test.go](../internal/domain/domain_test.go)

| Test Function | Type | What It Verifies |
| :--- | :--- | :--- |
| `TestMonotonicULID` | Unit | Generated ULIDs are non-empty, distinct, and strictly monotonic ascending within the same millisecond. |
| `TestProjectRefinementEligibility` | Unit | Includes `active` and `waiting` projects; excludes `on_hold`, `finished`, and `rejected` projects from Weekly Refinement. |
| `TestRoutineActiveCycles` | Unit | Deterministic cycle key calculation and automatic reset across all 5 schedule types: `daily`, `weekly` (ISO week rollover), `monthly`, `weekdays` (e.g., Mon/Wed/Fri), and `every_n_days` (interval rollover & pre-anchor suppression). |
| `TestShouldRemindRoutine` | Unit | Opt-in routine reminder obeys `reminder_time` threshold, fires once per active cycle, and suppresses repeat notifications within the same cycle. |
| `TestWeeklyRefinementEvaluation` | Unit | Weekly trigger calculation before/after scheduled time, catch-up after machine downtime, non-blocking banner state after first toast, snooze suppression & expiry re-notification, and clearing after completion. |
| `TestShouldRemindTask` | Unit | Opt-in task deadline reminder respects lead-time window and suppresses notifications for completed tasks. |

### 2.3 SQLite Storage, Migrations, Polymorphic Relations & Export (`internal/storage`)
Files: [internal/storage/store_test.go](../internal/storage/store_test.go), [internal/storage/export_md_test.go](../internal/storage/export_md_test.go)

| Test Function | Type | What It Verifies |
| :--- | :--- | :--- |
| `TestStoreProjectsTasksAndRefinementQuery` | Integration (SQLite) | Project & Task CRUD, `ListRefinementProjects` SQL filter, `ListDueReminderTasks` query, foreign-key cascade deletion of tasks on project delete, and polymorphic tag cleanup. |
| `TestIdeaPromotionWithLinksNotesResourcesAndHobbies` | Integration (SQLite) | Atomic `PromoteIdeaToProject` transaction: creates active Project, inserts initial tasks, copies Idea tags and bidirectional Note/Resource links onto the Project, links selected Hobbies, records `inspired_by` provenance link, and marks Idea `promoted`. |
| `TestPluginNamespacedMigrationAndPolymorphicEntities` | Integration (SQLite) | Idempotent namespaced plugin migration (`ext:people` creating `ext_people`) and polymorphic attachment of core Notes, Tags, and Progress Logs to plugin entity type `person` with zero core schema changes. |
| `TestRoutinesRefinementBackupAndExport` | Integration (WAL SQLite) | File-backed WAL DB open, `weekdays` routine persistence & cycle completion, refinement snooze/start/complete state transitions, online `VACUUM INTO` backup integrity check, and full `ExportJSON` snapshot round-trip. |
| `TestExportMarkdown` | Integration (FS) | Obsidian-compatible Markdown folder export (`projects/`, `ideas/`, `hobbies/`, `notes/`, `resources.md`, `routines.md`) with YAML frontmatter, task checklists, tags, and dated progress logs. |

### 2.4 Pluggable Extension System (`internal/ext`)
File: [internal/ext/ext_test.go](../internal/ext/ext_test.go)

| Test Function | Type | What It Verifies |
| :--- | :--- | :--- |
| `TestEventBusPubSubAndWildcard` | Unit | Event-specific and wildcard (`SubscribeAll`) delivery order and error handling. |
| `TestRegistryConfigToggleMigrationsAndHooks` | Integration (SQLite) | Disabled modules skip migrations/init/CLI hooks; enabling a module via `config.Plugins` runs its namespaced SQLite migrations (`ext:<id>`), subscribes to `EventBus`, and exposes CLI commands. |

### 2.5 External `$EDITOR` Helper (`internal/editor`)
File: [internal/editor/editor_test.go](../internal/editor/editor_test.go)

| Test Function | Type | What It Verifies |
| :--- | :--- | :--- |
| `TestResolveCommandPrecedenceAndQuoting` | Unit | Resolution order (`config.editor` → `$VISUAL` → `$EDITOR` → OS fallback), quoted Windows executable paths, and multi-argument flags (e.g. `code --wait`). |
| `TestPrepareTempFile` | Unit / FS | Temporary `.md` buffer creation and content round-trip. |

### 2.6 CLI Quick Capture & Subcommands (`internal/cli`)
File: [internal/cli/cli_test.go](../internal/cli/cli_test.go)

| Test Function | Type | What It Verifies |
| :--- | :--- | :--- |
| `TestCLIQuickCaptureListExportBackupAndPlugins` | End-to-End | Bare `pcrm add "..."` inbox shorthand with trailing flags, `add project|task|hobby|routine|note|link|log`, smart task-to-project resolution by title substring, `list`, `export --format json|markdown`, `backup`, `plugins enable`, and dynamic dispatch to enabled plugin CLI commands. |

---

## 3. Planned Automated Tests (Upcoming Phases)

### Phase 4 — Bubble Tea TUI (`internal/tui`)
- Root tab switching & dynamic injection of enabled plugin tabs.
- Dashboard rendering with active Weekly Refinement banner, due routine one-keystroke toggle, and snooze action.
- Projects & Tasks interactive state transitions (status cycle, add/complete task, add dated progress log).
- Ideas Tracker guided Promote-to-Project modal flow.
- Notes/Resources linking and `$EDITOR` command handoff (`tea.ExecProcess`).
- Guided Weekly Refinement multi-step wizard (Projects → Ideas → Routines → completion persistence).

### Phase 5 — Background Tray Daemon, Scheduler & Windows Integration (`internal/daemon`, `internal/platform`)
- Scheduler tick tests using injected fake clock + mock `Notifier` (Weekly Refinement catch-up, snooze re-notify, opt-in Routine & Task deadline reminders).
- Single-instance IPC lock & daemon control commands (start, status, focus/open TUI, stop).
- Windows autostart registry/task toggle verification & portable-mode detection test.

### Phase 6 — Cross-Platform & First Optional Plugins
- Linux/macOS `internal/platform` build compilation checks (`GOOS=linux`, `GOOS=darwin`) and unit tests for path/autostart generators.
- End-to-end tests for initial pluggable modules (e.g., `people`, `reports`).
