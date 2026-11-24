# PersonalCRM — Project Context & Product Specification

> **Status:** Confirmed (v1 Scope Locked)  
> **Last Updated:** 2026-11-24

---

## 1. Product Vision & Philosophy

PersonalCRM (`pcrm`) is a lightweight, local-first **Personal Knowledge + Project Management TUI** tailored for personal life (not a sales CRM), inspired by a pragmatic hybrid of **PARA** (Projects, Areas/Hobbies, Resources, Archives) and **light GTD** (Capture → Clarify → Weekly Review). Its purpose is to:

- Reduce mental load by giving every active project, half-formed idea, hobby, and routine a clear home.
- Keep ideas alive with reusable research notes and reference links until they are ready to become projects — **prefer parking/archiving over deleting** so active lists stay focused without losing past thinking.
- Sustain momentum through a scheduled **Weekly Refinement** loop triggered via OS notifications.
- Stay unobtrusively alive in the system tray (with optional OS autostart) and allow instant keyboard-driven capture and management.
- Grow gracefully over time via a **pluggable architecture** (allowing future modules such as People, Reports, or integrations to be added and enabled cleanly).

---

## 2. Target Platforms, Packaging & Runtime Model

| Aspect | Confirmed Decision |
| :--- | :--- |
| **Interface** | Keyboard-driven TUI (`Bubble Tea`) + CLI quick-capture |
| **Tech Stack** | **Go** (pure-Go SQLite via `modernc.org/sqlite`, Bubble Tea + Lip Gloss + Bubbles) — see [RUST_VS_GO.md](RUST_VS_GO.md) |
| **Primary OS (v1)** | **Windows** — distributed as a **portable single `.exe`** (`pcrm.exe`) |
| **Secondary OS targets** | **Debian Linux**, **macOS** (enabled via clean OS abstraction layer) |
| **Tray & Window Model** | Single binary with **background tray daemon + terminal TUI client**. Tray stays alive until explicit Quit; clicking tray icon or notification opens/focuses a terminal running the TUI; closing the terminal leaves the tray alive. |
| **Autostart** | Optional, toggled in app settings (Windows first; Linux/macOS backends designed in interface) |
| **Storage** | Local **SQLite** database as single source of truth + built-in Markdown/JSON export & backup |
| **Note Editing** | Quick inline editing for titles/short fields; **external `$EDITOR`** for longer notes & descriptions |

---

## 3. Core Domain Model

### 3.1 Four Core Pillars

1. **Projects**
   - *Definition:* Scoped activities aimed at achieving a concrete future result or milestone.
   - *Fields:* Title, description, goal/desired outcome, status, priority (`low`, `medium`, `high`), start date, target date, finished date, tasks (open/completed next actions), dated progress logs, linked notes, linked resources/links, linked hobbies, tags.
   - *Statuses:* `active`, `waiting`, `on_hold`, `finished`, `rejected`.
   - *Refinement rule:* Included in Weekly Refinement whenever status is **not** `finished`, `rejected`, or `on_hold` (i.e., `active` and `waiting`).

2. **Ideas Tracker (also serves as Inbox)**
   - *Definition:* Fast capture inbox + "not started" projects with research context. No heavy structure required at capture time.
   - *Fields:* Title, description, status (`inbox`, `exploring`, `ready`, `parked`, `promoted`, `discarded`), source (e.g., conversation, walk, article), excitement/energy score (optional 1–5), capture timestamp, linked notes, linked notes-to-notes, linked resources/URLs for research, related hobbies, tags.
   - *Key workflow — **Promote to Project**:* Creates a new Project carrying over title/description and linking associated notes, resources, and tags, while marking the idea `promoted` and recording `promoted_project_id`. The guided promotion form prompts for:
     - **Desired outcome** (what success looks like / why it matters)
     - **Constraints & resources needed** (optional notes/links)
     - **Initial next actions** (first 1–5 concrete tasks)
     - **Related hobbies/areas**

3. **Hobbies**
   - *Definition:* Ongoing areas of interest with **no fixed end target**.
   - *Fields:* Name, description, current focus, status (`active`, `on_hold`, `rejected`), dated progress/reflection logs (what happened, what was learned, engagement notes), linked notes, linked resources/links, linked projects/ideas, tags.

4. **Routines**
   - *Definition:* Simple checklist of recurring activities that reset cleanly on schedule.
   - *Schedules supported:*
     - `daily`
     - `weekly`
     - `monthly`
     - `every_n_days` (custom interval in days from an anchor date)
     - `weekdays` (specific days of the week, e.g., Mon/Wed/Fri)
   - *Behavior:* Simple resettable checklist per cycle (tracks completion for the active cycle without heavy streak/history ledgers). Supports pause/active state and opt-in due reminder notifications.

### 3.2 Standalone Reusable Supporting Entities

- **Tasks (Actions):** Concrete next steps belonging to a Project (title, status: `open`/`in_progress`/`blocked`/`done`/`cancelled`, priority, optional due date, opt-in deadline reminder).
- **Notes:** Standalone reusable Markdown notes (many-to-many linkable to Projects, Ideas, Hobbies, other Notes, and future plugin entities). Edited via external `$EDITOR`.
- **Resources / Links:** Standalone reusable reference items (URLs, books, courses, tools, local paths) with status (`to_consume`, `in_progress`, `reference`) and brief notes, many-to-many linkable to any entity.
- **Progress Logs:** Chronological dated progress/reflection entries ("what happened, what I learned, what changed") attached polymorphically to Projects and Hobbies (and available to future plugin entities).
- **Tags:** Lightweight user-defined labels attachable to any entity.
  - *Recommended optional tag conventions (from PARA/GTD):* Energy (`energy:high|medium|low`), Time horizon (`horizon:week|month|someday`), Domain (`domain:tech|creative|physical|social|learning`), Context (`ctx:home|outside|computer|phone`).

### 3.3 Deferred to Pluggable Extensions

- **People** (contacts relevant to projects/hobbies, context, last interaction, notes) — deferred to an optional plugin/extension module.
- **Reports / Analytics** — deferred to an optional plugin/extension module.
- **Future integrations** (e.g., Obsidian/Markdown sync, calendar feeds, webhooks) — designed to plug into the extension system.

---

## 4. Core Workflows

### 4.1 Dashboard & Daily Use
- Opens to a clean **Dashboard** showing:
  - **Prominent Weekly Refinement banner** whenever a refinement cycle is due (press key to start Refinement or snooze).
  - Due **Routines** for the current cycle (one-keystroke checkoff).
  - Active **Projects** and their open next actions.
  - Quick navigation tabs across **Dashboard**, **Projects**, **Ideas**, **Hobbies**, **Routines**, **Notes & Links**, plus any enabled plugin tabs.

### 4.2 Fast Capture (Without Opening Full TUI)
- **CLI commands** for instant capture from any terminal or launcher:
  - `pcrm add idea "Title" [-d "desc"] [-t tag1,tag2]`
  - `pcrm add task "Title" --project <id|name> [--due YYYY-MM-DD]`
  - `pcrm add note "Title" [--link project:<id>]` (optionally opens `$EDITOR`)
  - `pcrm add link "https://..." [--title "..."] [--link idea:<id>]`
- **System Tray quick menu:** Open TUI, Start Refinement (when due), Quick Capture shortcut, Autostart toggle, Quit.

### 4.3 Weekly Refinement & Notifications
- **Schedule:** Configurable weekly day & time (default e.g. Sunday 18:00).
- **Catch-up:** If the machine was off or asleep at the scheduled time, the daemon detects the overdue cycle on next startup/wake and sends the OS notification until Refinement is completed or snoozed.
- **Entry UX:** Opening the app shows the normal Dashboard with a **prominent banner** to launch Refinement (never traps the user if they just needed to check a routine quickly).
- **Guided Refinement Steps:**
  1. **Projects & Open Tasks:** Walk through every project where status is not `finished`, `rejected`, or `on_hold` — update status, mark tasks done, add next actions, or log a quick progress entry.
  2. **Ideas Triage:** Review `inbox` and `exploring` ideas — enrich, park, discard, or promote to an active Project.
  3. **Routines Check:** Review active routines — check off due items, adjust schedules, or pause/reactivate routines.
  4. *(Extensible)* Enabled plugins may append optional refinement steps.
- **Additional Notifications (Opt-In):**
  - Due **Routine** reminders (per-routine opt-in time/toggle + global setting).
  - **Task deadline** reminders (per-task or global opt-in before due date/time).

---

## 5. Non-Functional Requirements

- **Build once, extend cleanly:** Stable SQLite schema with polymorphic links/tags/logs and a formal extension registry so future features never require rewriting core code.
- **Portable & low footprint:** Single `.exe` on Windows; tiny idle RAM/CPU footprint while resident in the system tray.
- **Keyboard-first:** Full functionality via keyboard shortcuts (optional mouse support as a bonus).
- **Safe local data:** Atomic SQLite transactions, WAL mode, versioned forward migrations, and built-in `pcrm export` / `pcrm backup`.
