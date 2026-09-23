CREATE TABLE IF NOT EXISTS tasks (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    company            TEXT    NOT NULL DEFAULT '',
    title              TEXT    NOT NULL DEFAULT '',
    industry           TEXT    NOT NULL DEFAULT '',
    category           TEXT    NOT NULL DEFAULT ''
        CHECK (category IN ('', 'crm', 'automation', 'analytics', 'ai_assistant',
                            'web_app', 'integration', 'content', 'other')),
    draft_text         TEXT    NOT NULL DEFAULT '',
    qa                 TEXT    NOT NULL DEFAULT '[]',
    context            TEXT    NOT NULL DEFAULT '',
    need               TEXT    NOT NULL DEFAULT '',
    users              TEXT    NOT NULL DEFAULT '',
    data               TEXT    NOT NULL DEFAULT '',
    constraints        TEXT    NOT NULL DEFAULT '',
    expected_result    TEXT    NOT NULL DEFAULT '',
    success_criteria   TEXT    NOT NULL DEFAULT '',
    contact            TEXT    NOT NULL DEFAULT '',
    interaction_format TEXT    NOT NULL DEFAULT '',
    confirmed          TEXT    NOT NULL DEFAULT '[]',
    status             TEXT    NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'published')),
    score              INTEGER NOT NULL DEFAULT 0,
    created_at         TEXT    NOT NULL DEFAULT '',
    published_at       TEXT
);

CREATE INDEX IF NOT EXISTS idx_tasks_status_score ON tasks (status, score DESC);

CREATE TABLE IF NOT EXISTS teams (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    name      TEXT NOT NULL DEFAULT '',
    interests TEXT NOT NULL DEFAULT '[]',
    skills    TEXT NOT NULL DEFAULT '',
    tech      TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS proposals (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id            INTEGER NOT NULL REFERENCES tasks (id),
    team_id            INTEGER NOT NULL REFERENCES teams (id),
    idea               TEXT    NOT NULL DEFAULT '',
    plan               TEXT    NOT NULL DEFAULT '',
    deadline           TEXT    NOT NULL DEFAULT '',
    prototype_url      TEXT    NOT NULL DEFAULT '',
    status             TEXT    NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'accepted', 'rejected')),
    decided_at         TEXT,
    stage_confirmed_at TEXT,
    points_awarded     INTEGER NOT NULL DEFAULT 0,
    created_at         TEXT    NOT NULL DEFAULT ''
);
