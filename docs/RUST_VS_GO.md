# Tech Stack Rationale: Rust vs. Go (Reference Guide)

> **Purpose:** Captures the trade-offs evaluated when choosing the language for **PersonalCRM** (`pcrm`), written as a reusable reference for future CLI, TUI, and system-tray/daemon projects.  
> **Project Decision:** **Go** (`ADR-002` — see [STATUS.md](STATUS.md)).  
> **Date:** 2026-11-24

---

## 1. Quick Comparison Matrix

| Dimension | Rust (`ratatui` + `tray-icon` + `rusqlite`) | Go (`bubbletea` + `systray` + `modernc.org/sqlite`) |
| :--- | :--- | :--- |
| **Idle RAM (always-on daemon)** | ~3–6 MB (no GC/runtime) | ~10–20 MB (tiny on modern OSes) |
| **Single binary size** | ~5–8 MB (release/stripped) | ~15–25 MB (incl. pure-Go SQLite) |
| **TUI Developer Experience** | Good (`ratatui` — immediate-mode, fine-grained control, more layout/state boilerplate) | **Excellent** (`Bubble Tea` + `Lip Gloss` + `Bubbles` — Elm architecture, clean modals/forms/`$EDITOR` suspension) |
| **Pluggable / Modular Architecture** | Strong type safety, but trait objects, lifetimes, and shared mutable state (`Arc<Mutex<...>>`) add friction | **Very clean** implicit interfaces for registries, event buses, and cross-platform adapters |
| **Desktop OS Integration (Tray, Toast, GUI)** | **Stronger** — battle-tested Tauri ecosystem crates (`tray-icon`, `muda`, `windows-rs`, `notify-rust`) + native GUI (`egui`, `iced`) | Good via smaller community packages; pure-Go on Windows/Linux (`fyne.io/systray`), CGO needed on macOS tray |
| **SQLite Story (Windows portable `.exe`)** | Clean C SQLite bundled via `rusqlite` (`bundled` feature) | Zero-CGO pure-Go SQLite via `modernc.org/sqlite` (fast enough for sub-ms CRM queries, slightly larger binary) |
| **Build Speed & Cross-Build** | Slower cold/incremental compiles; cross-compiling C deps takes care | **Very fast** compiles; `CGO_ENABLED=0 go build` is effortless on Windows/Linux |

---

## 2. Why Rust Was Initially Ranked #1 (Before Full Requirements)

Before locking in the terminal-TUI + pluggable-module architecture, Rust had four structural advantages:

1. **Minimal always-on daemon footprint:** For a background process running from OS login to shutdown, Rust's lack of a garbage collector gives the lowest possible memory and binary footprint.
2. **Mature native desktop crates:** The Tauri ecosystem maintains high-quality, unified cross-platform bindings for system tray menus, WinRT/Win32 APIs, and OS notifications.
3. **Bundled native C SQLite:** `rusqlite` compiles official C SQLite cleanly without forcing a pure-language port or manual MinGW/CGO setup on Windows.
4. **Native GUI fallback:** If the project had needed a standalone native GUI window rendered like a TUI (Option `1B`), Rust's GUI ecosystem (`egui`, `iced`) is much stronger than Go's.

---

## 3. Why Go Is the Better Fit for PersonalCRM (Locked Requirements)

Once the final architecture was confirmed — **terminal TUI + background tray daemon (`1A`)**, **pluggable extension system**, **external `$EDITOR` workflow**, and **portable single `.exe` on Windows** — Go's strengths directly matched the project's biggest complexity areas:

1. **Best-in-class TUI ergonomics:** Charm's Bubble Tea stack (Model → Update → View) makes multi-view navigation, prominent banners, forms, and suspending the terminal for `$EDITOR` (`tea.ExecProcess`) dramatically simpler to build and evolve.
2. **Low-friction pluggable architecture:** Writing `ext.Module`, polymorphic storage repositories, an event bus, and OS `platform` interfaces in Go stays concise and readable without fighting borrow-checker lifetimes across callbacks and plugin registries.
3. **Trivial zero-CGO Windows builds:** `CGO_ENABLED=0 go build` produces a single portable `pcrm.exe` in seconds with no C compiler required.
4. **Negligible real-world runtime trade-offs:** ~15 MB idle RAM and sub-millisecond `modernc.org/sqlite` queries on personal-scale data (hundreds to tens of thousands of records) are indistinguishable from native C/Rust in daily use.

---

## 4. Rule of Thumb for Future Projects

- **Pick Rust when:**
  - You need a **native GUI window** (or embedded terminal renderer) with custom graphics.
  - Ultra-low memory/binary size, hard real-time latency, or heavy CPU/data-processing throughput is a primary requirement.
  - Deep native OS / WinRT / COM / audio / GPU integration is central to the product.
- **Pick Go when:**
  - You are building a **rich terminal TUI**, **CLI tool**, **local-first productivity app**, or **lightweight daemon/service**.
  - Fast iteration, clean interfaces/modularity, simple concurrency, and effortless single-binary compilation matter more than shaving 10 MB of RAM.
