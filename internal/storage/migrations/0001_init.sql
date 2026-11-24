-- PersonalCRM Core Schema v1

CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    goal        TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'active'
                CHECK (status IN ('active', 'waiting', 'on_hold', 'finished', 'rejected')),
    priority    TEXT NOT NULL DEFAULT 'medium'
                CHECK (priority IN ('low', 'medium', 'high')),
    start_date  TEXT,
    target_date TEXT,
    finished_at TEXT,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tasks (
    id               TEXT PRIMARY KEY,
    project_id       TEXT REFERENCES projects(id) ON DELETE CASCADE,
    title            TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'open'
                     CHECK (status IN ('open', 'in_progress', 'blocked', 'done', 'cancelled')),
    priority         TEXT NOT NULL DEFAULT 'medium'
                     CHECK (priority IN ('low', 'medium', 'high')),
    due_at           TEXT,
    reminder_enabled INTEGER NOT NULL DEFAULT 0,
    reminder_sent_at TEXT,
    position         INTEGER NOT NULL DEFAULT 0,
    completed_at     TEXT,
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS ideas (
    id                  TEXT PRIMARY KEY,
    title               TEXT NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    status              TEXT NOT NULL DEFAULT 'inbox'
                        CHECK (status IN ('inbox', 'exploring', 'ready', 'parked', 'promoted', 'discarded')),
    source              TEXT NOT NULL DEFAULT '',
    excitement          INTEGER CHECK (excitement IS NULL OR (excitement BETWEEN 1 AND 5)),
    promoted_project_id TEXT REFERENCES projects(id) ON DELETE SET NULL,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS hobbies (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    current_focus TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'active'
                  CHECK (status IN ('active', 'on_hold', 'rejected')),
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS routines (
    id                  TEXT PRIMARY KEY,
    title               TEXT NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    schedule_type       TEXT NOT NULL
                        CHECK (schedule_type IN ('daily', 'weekly', 'monthly', 'every_n_days', 'weekdays')),
    schedule_config     TEXT NOT NULL DEFAULT '{}',
    active              INTEGER NOT NULL DEFAULT 1,
    reminder_enabled    INTEGER NOT NULL DEFAULT 0,
    reminder_time       TEXT NOT NULL DEFAULT '',
    completed_cycle_key TEXT NOT NULL DEFAULT '',
    last_completed_at   TEXT,
    last_reminded_cycle TEXT NOT NULL DEFAULT '',
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS notes (
    id          TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    content     TEXT NOT NULL DEFAULT '',
    source      TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS resources (
    id            TEXT PRIMARY KEY,
    title         TEXT NOT NULL,
    url_or_path   TEXT NOT NULL,
    resource_type TEXT NOT NULL DEFAULT 'link'
                  CHECK (resource_type IN ('link', 'book', 'course', 'tool', 'file', 'other')),
    status        TEXT NOT NULL DEFAULT 'reference'
                  CHECK (status IN ('to_consume', 'in_progress', 'reference')),
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS entity_links (
    source_type   TEXT NOT NULL,
    source_id     TEXT NOT NULL,
    target_type   TEXT NOT NULL,
    target_id     TEXT NOT NULL,
    relation_type TEXT NOT NULL DEFAULT 'related',
    created_at    TEXT NOT NULL,
    PRIMARY KEY (source_type, source_id, target_type, target_id, relation_type)
);

CREATE TABLE IF NOT EXISTS progress_logs (
    id          TEXT PRIMARY KEY,
    entity_type TEXT NOT NULL,
    entity_id   TEXT NOT NULL,
    logged_at   TEXT NOT NULL,
    content     TEXT NOT NULL,
    created_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tags (
    id   TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE
);

CREATE TABLE IF NOT EXISTS entity_tags (
    entity_type TEXT NOT NULL,
    entity_id   TEXT NOT NULL,
    tag_id      TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (entity_type, entity_id, tag_id)
);

CREATE TABLE IF NOT EXISTS refinement_runs (
    id                TEXT PRIMARY KEY,
    cycle_key         TEXT NOT NULL,
    started_at        TEXT NOT NULL,
    completed_at      TEXT,
    projects_reviewed INTEGER NOT NULL DEFAULT 0,
    ideas_reviewed    INTEGER NOT NULL DEFAULT 0,
    routines_reviewed INTEGER NOT NULL DEFAULT 0,
    notes             TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS app_state (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status);
CREATE INDEX IF NOT EXISTS idx_tasks_project_status ON tasks(project_id, status);
CREATE INDEX IF NOT EXISTS idx_tasks_due ON tasks(due_at) WHERE status NOT IN ('done', 'cancelled');
CREATE INDEX IF NOT EXISTS idx_ideas_status ON ideas(status);
CREATE INDEX IF NOT EXISTS idx_hobbies_status ON hobbies(status);
CREATE INDEX IF NOT EXISTS idx_links_source ON entity_links(source_type, source_id);
CREATE INDEX IF NOT EXISTS idx_links_target ON entity_links(target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_logs_entity ON progress_logs(entity_type, entity_id, logged_at DESC);
CREATE INDEX IF NOT EXISTS idx_entity_tags_lookup ON entity_tags(entity_type, entity_id);
