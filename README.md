# PersonalCRM (`pcrm`)

**A lightweight, keyboard-driven, local-first personal CRM for your Projects, Ideas, Hobbies, and Routines — right in your terminal.**

PersonalCRM is built for personal life rather than sales pipelines. It keeps your active projects moving, gives half-formed ideas a home with linked notes and research links, tracks ongoing hobbies without forcing arbitrary deadlines, resets simple recurring routines automatically, and guides you through a **Weekly Refinement** check-in so nothing quietly goes stale.

- **100% local & private:** Everything stays on your machine (single portable executable, no cloud account required).
- **Fast capture anywhere:** Dump ideas, tasks, notes, or links in one command from any terminal, or manage everything interactively in the TUI.
- **Cross-platform:** Pre-built standalone executables for **Windows**, **Ubuntu / Debian Linux**, and **macOS** (Intel & Apple Silicon).

---

## Core Pillars

| Area | What it's for |
| :--- | :--- |
| **Projects** | Scoped efforts with a desired outcome, priority, open tasks, dated progress logs, and linked notes/links. |
| **Ideas Tracker** | Your fast-capture inbox and research space for not-yet-started projects. Attach notes and reference URLs, then **promote** an idea to a Project in one step (carrying over all links and tags). |
| **Hobbies** | Ongoing interests with no finish line (`active`, `on_hold`, `rejected`), a current focus, and dated reflection/progress logs. |
| **Routines** | A simple resetting checklist supporting **daily**, **weekly**, **monthly**, **every N days**, and **specific weekdays** (e.g. Mon/Wed/Fri). |
| **Notes & Links** | Standalone reusable Markdown notes (edited in your favorite external editor) and reference links/books/tools that can be linked across multiple Projects, Ideas, and Hobbies. |
| **Weekly Refinement** | A guided review over your active Projects & open tasks, inbox Ideas, and active Routines — surfaced via a non-blocking banner on the Dashboard whenever a weekly check-in is due. |

---

## Installation

Download the latest release binary for your OS from the **[Releases page](https://github.com/ViacheslavK/PersonalCRM/releases/latest)** (checksums are included in `SHA256SUMS.txt`).

### Windows (Portable `.exe`)
1. Download `pcrm-vX.Y.Z-windows-amd64.exe` and rename it to `pcrm.exe`.
2. Put `pcrm.exe` in any folder (for example, `C:\Tools\pcrm\`) and optionally add that folder to your `PATH`.
3. Run `pcrm.exe` in Windows Terminal or PowerShell.

> **Portable Mode (USB / Sync Folder):** Create an empty file named `pcrm.portable` next to `pcrm.exe` to store all data, backups, and `config.toml` in a `./data/` folder right beside the executable instead of `%APPDATA%\PersonalCRM`.

### Ubuntu / Debian Linux
```bash
# Download the latest linux-amd64 binary from Releases, then:
chmod +x pcrm-v*-linux-amd64
sudo mv pcrm-v*-linux-amd64 /usr/local/bin/pcrm
pcrm version
```

### macOS (Apple Silicon & Intel)
- Apple Silicon (M1/M2/M3/M4): download `pcrm-v*-darwin-arm64`
- Intel Mac: download `pcrm-v*-darwin-amd64`

```bash
chmod +x pcrm-v*-darwin-*
sudo mv pcrm-v*-darwin-* /usr/local/bin/pcrm
# If macOS Gatekeeper prompts on first run:
xattr -d com.apple.quarantine /usr/local/bin/pcrm
pcrm version
```

### Build from Source (Optional)
Requires Go 1.22+ (`CGO_ENABLED=0` — no C compiler needed):
```bash
git clone https://github.com/ViacheslavK/PersonalCRM.git
cd PersonalCRM
CGO_ENABLED=0 go build -o pcrm ./cmd/pcrm
```

---

## Quick Start & Everyday Use Cases

### 1. Capture Thoughts Instantly from Any Terminal (CLI)

You don't need to open the full interface just to save a spark of inspiration or a link:

```bash
# Bare shorthand — dumps straight into your Ideas Tracker (inbox)
pcrm add "Build a DIY split ergonomic keyboard" -t hardware,diy -e 5

# Create a Project and add concrete tasks (matches project by name or ID)
pcrm add project "Home Lab Upgrade" --goal "Quiet low-power home server" --priority high --target 2026-12-31
pcrm add task "Compare N100 mini PCs" --project "Home Lab" --due 2026-12-01 --remind

# Track a Hobby (ongoing, no finish line)
pcrm add hobby "Photography" --focus "Street portraits" -t creative

# Add Routines with flexible schedules
pcrm add routine "Morning Stretch" --schedule daily
pcrm add routine "Strength Workout" --schedule weekdays --days mon,wed,fri --remind 07:30
pcrm add routine "Water houseplants" --schedule every_n_days --every 3

# Capture standalone Notes & Links (optionally link them right away)
pcrm add note "ZMK Firmware Notes" --edit -t hardware
pcrm add link "https://github.com/foostan/crkbd" --title "Corne Keyboard Repo" --status to_consume

# Log dated progress on a Project or Hobby
pcrm add log project:<id> "Benchmarked idle power draw at 8W"

# Quick terminal lists
pcrm list projects
pcrm list ideas
pcrm list routines
```

### 2. Manage Your Day in the Interactive TUI

Launch the TUI anytime by running:
```bash
pcrm
```

#### Common TUI Workflows
- **Morning check-in (`1` Dashboard):** Check off today's due Routines with `Space`, scan your Active Projects and top open next actions, and see immediately if a **Weekly Refinement** banner is waiting (`R` to start, `S` to snooze).
- **Working on a Project (`2` Projects):** Press `→` to jump into a project's Tasks, `Space` to mark tasks done, `t` to add a next action, `l` to record a dated progress log, `c` to link existing Notes or Links, `e` to edit longer descriptions in your external editor, and `f` to filter (`Active & Waiting` / `All` / `On Hold` / `Finished & Rejected`).
- **Turning an Idea into a Project (`3` Ideas):** Research an idea over time by attaching standalone Notes and Links (`c`). When it's ready, press **`P`** to open the guided promotion form (goal, priority, initial tasks) — PersonalCRM creates the active Project and automatically carries over all linked notes, links, and tags.
- **Reflecting on Hobbies (`4` Hobbies):** Keep track of what you're currently focusing on, log dated sessions/reflections with `l`, and switch between `active`, `on_hold`, and `rejected` with `s`.
- **Writing Long Notes (`6` Notes & Links):** Press `e` (or `Enter`) on any Note to edit full Markdown in your preferred external editor (configured via `config.toml`, `$VISUAL`, `$EDITOR`, or falling back to Notepad / `nano` / `vi`). Press `v` to switch between **Notes** and **Resources/Links**.
- **Running Weekly Refinement (`R`):** Walks you step-by-step through:
  1. **Active & Waiting Projects** (excluding `finished`, `rejected`, and `on_hold`) — check off tasks, add next actions, log progress, or update project statuses.
  2. **Ideas Triage** — park, explore, discard, or promote (`P`) ideas.
  3. **Routines Check** — check off or pause routines.
  4. **Summary** — press `Enter` to finish and clear the Dashboard banner until next week.

#### TUI Keyboard Reference

| Keys | Action |
| :--- | :--- |
| `1`–`6` / `Tab` / `Shift+Tab` | Switch between Dashboard, Projects, Ideas, Hobbies, Routines, Notes & Links |
| `j` / `k` or `↑` / `↓` | Navigate lists |
| `←` / `→` | Switch panes (Projects ↔ Tasks) or Notes ↔ Links |
| `Space` / `Enter` | Toggle Task or Routine completion |
| `n` | Create new item in current tab |
| `t` | Add a Task to the selected Project |
| `s` / `p` | Cycle status / priority |
| `l` | Add a dated Progress / Reflection log |
| `c` | Connect (link) a standalone Note, Link, Hobby, Project, or Idea |
| `g` | Add comma-separated tags |
| `e` | Open description or note in external `$EDITOR` |
| `P` | Promote Idea to Project (in Ideas tab & Weekly Refinement) |
| `R` / `S` | Start or Snooze Weekly Refinement |
| `f` | Cycle list filter (Projects / Ideas) |
| `r` | Refresh view from disk (loads items added via CLI) |
| `d` | Delete selected item (asks for confirmation) |
| `?` / `q` | Toggle shortcut help / Quit |

---

## Configuration, Backups & Exports

Your data stays completely under your control:

```bash
# See where your database, config, and backups live
pcrm paths

# Use an isolated/custom data folder for any command
pcrm --data-dir ./my-data

# Create a safe backup copy of your database
pcrm backup
pcrm backup --out ./backups/my-crm-backup.db

# Export everything to JSON or an Obsidian-compatible Markdown folder
pcrm export --format json --out ./export.json
pcrm export --format markdown --out ./my-vault/
```

### Customizing `config.toml`
Run `pcrm paths` to find your `config.toml` (auto-created when settings are saved). Example configuration:

```toml
editor = "code --wait" # Or "nvim", "notepad.exe", etc.

[refinement]
enabled = true
weekday = "sunday"
time = "18:00"
snooze_hours = 24

[notifications]
refinement = true
routines = false       # Opt-in
task_deadlines = false # Opt-in
task_lead_minutes = 60
```

---

## Roadmap

- **v0.4.0 (Current):** Core storage, CLI quick-capture, JSON & Markdown export, backup, full interactive Bubble Tea TUI, external `$EDITOR` workflow, guided Weekly Refinement, and pluggable module registry.
- **Next (`v0.5.0`):** Background system tray daemon, OS notifications (Weekly Refinement + opt-in Routine & Task deadline reminders), and login autostart (Windows first).
- **Upcoming (`v0.6.0+`):** Native Debian Linux & macOS tray/notification backends, plus optional toggleable modules such as **People & Actors** (customers, external workers, family/friends, and AI agents reading/updating tasks via CLI) and **Reports**.
