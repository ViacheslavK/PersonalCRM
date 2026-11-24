# PersonalCRM — Pluggable Module Spec: People & Actors (`ext:people`)

> **Status:** Confirmed Design (Scheduled for Phase 6; architectural hooks built into Phases 2–4)  
> **Module ID:** `people` (`ext:people`)  
> **Last Updated:** 2026-11-24

---

## 1. Vision & Purpose

The `people` module extends PersonalCRM with a unified directory of **People, Organizations, and AI Agents** ("actors") involved in personal projects, hobbies, ideas, and tasks. It is designed to plug cleanly into core storage and UI via `internal/ext` without modifying core tables when disabled.

---

## 2. Taxonomy & Categories

Actors are organized along two clean dimensions — **Scope** (`external` vs. `internal`) and **Actor Kind / Role**:

| Scope | Category / Role | Examples & Usage |
| :--- | :--- | :--- |
| **External** | **Person / Company** | General external contacts, vendors, organizations, mentors. Individuals can link to companies (`works_at`). |
| **External** | **Customer** | Source of requirements / stakeholder for a Project (`project --(customer)--> person`). |
| **External** | **External Worker** | Contractor / freelancer responsible for implementing specific tasks (`task --(assigned_to)--> person`), with optional **contract / payment agreement** details. |
| **Internal** | **Family / Friends** | Close personal contacts linked to hobbies, personal projects, shared ideas, or interaction logs. |
| **Internal** | **AI Agent** | Autonomous/assisted coding or research agents (e.g., Claude, Antigravity, Codex) that can be assigned project tasks and consume/update them via CLI. |

---

## 3. Core Workflows & Integration Points

### 3.1 Zero-Core-Schema-Change Polymorphic Linking
Because PersonalCRM's core `entity_links`, `entity_tags`, and `progress_logs` tables use open `(entity_type, entity_id)` references, `ext:people` uses `entity_type = 'person'` and immediately gains:
- **Project ↔ Customer / Stakeholder:** `entity_links (project:<id>, person:<id>, 'customer' | 'stakeholder')`
- **Task ↔ Assignee (External Worker or AI Agent):** `entity_links (task:<id>, person:<id>, 'assigned_to')`
- **Person ↔ Company:** `entity_links (person:<id>, person:<company_id>, 'works_at')`
- **Notes, Resources/Links & Tags:** Standalone Notes, contract links/files, and tags attach to any person/company/agent automatically.
- **Interaction Logs:** Dated entries in `progress_logs (entity_type = 'person', entity_id = <id>)` track last interactions, calls, or agent handoffs.

### 3.2 External Worker Contracts & Agreements (Plugin-Owned Tables)
Owned inside `ext:people` namespaced migrations (`ext_people`, `ext_people_agreements`):
- `ext_people`: `id`, `name`, `handle` (short CLI slug like `claude`, `alex`), `scope` (`external` | `internal`), `kind` (`person` | `company` | `ai_agent`), `default_category` (`customer` | `external_worker` | `family` | `friend` | `ai_agent` | `contact`), `contact_info` (email/phone/url), `notes`, `status`, `created_at`, `updated_at`.
- `ext_people_agreements`: optional contract/payment agreement records linked to `(person_id, project_id)` storing scope summary, pricing/rate & currency, payment terms, agreement status (`draft` | `active` | `paid` | `closed`), and reference links/notes.

### 3.3 AI Agent Task-by-Task CLI Workflow
A flagship workflow for `ai_agent` actors:
1. User creates a Project and ordered list of Tasks (`tasks.position` + Markdown `tasks.description` + linked research Notes/Resources).
2. User assigns one or more tasks to an AI agent actor (e.g., handle `claude`).
3. The AI agent (e.g., Claude Code) reads its queue via machine-readable CLI:
   - `pcrm people tasks claude [--project <ref>] [--status open] --json`
   - Fetches ordered tasks with full description, linked notes, and linked resources.
4. As the agent works task-by-task, it updates progress via CLI:
   - `pcrm task update <id> --status in_progress`
   - `pcrm add log project:<id> "Implemented step 1..."`
   - `pcrm task update <id> --status done`
5. Updates appear immediately in the user's TUI and Weekly Refinement (shared SQLite WAL database).

---

## 4. Architectural Safeguards (Applied Before Phase 4)

To ensure `ext:people` drops in cleanly in Phase 6 without refactoring Phases 2–5:
- **Core storage:** Keep `entity_links` as the canonical relationship table for assignments and project roles (no hardcoded `person_id` foreign keys in core tables).
- **Core CLI:** Design list/task-update commands to support `--json` output cleanly so AI agents can reliably parse structured task queues.
- **Core TUI (Phase 4):** Include clean extension points in `internal/tui` for:
  - Additional top-level plugin tabs (`Registry.TUIViewSpecs()`).
  - Optional detail-panel sections / item badges (e.g., `@assignee` tag on tasks and Customer section on projects when `people` is enabled).

---

## 5. Confirmed Design Decisions

1. **Multi-role contacts:** Confirmed — a single person or company may hold multiple roles across projects (modeled via default category + per-link `relation_type` such as `customer`, `assigned_to`, `stakeholder`, `works_at`).
2. **Lightweight external worker agreements:** Confirmed — simple status/check tracking per `(worker, project)` (scope summary, rate/fixed amount + currency, status `draft`/`active`/`paid`/`closed`, due date, and notes/link to contract) rather than a full invoicing ledger.
3. **AI agent read & write-back CLI workflow:** Confirmed — agents read ordered assigned tasks in structured JSON (using Markdown `task.description` + linked Notes/Resources as task context) and write back progress via CLI (`task update`, `add log`).
4. **TUI `@handle` badges & project section:** Confirmed — when `people` is enabled, Tasks display a subtle `@handle` badge and Projects show linked Customer/People details via `internal/ext` UI hooks; completely hidden when disabled.
