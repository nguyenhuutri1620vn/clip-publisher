CREATE TABLE IF NOT EXISTS videos (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    slug            TEXT NOT NULL,
    title           TEXT NOT NULL DEFAULT '',
    origin          TEXT NOT NULL,
    source_path     TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL,
    status_detail   TEXT NOT NULL DEFAULT '',
    voice_profile   TEXT NOT NULL DEFAULT '',
    music           TEXT NOT NULL DEFAULT '',
    current_version INTEGER NOT NULL DEFAULT 0,
    editable        INTEGER NOT NULL DEFAULT 1,
    hidden          INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS versions (
    video_id    INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    n           INTEGER NOT NULL,
    script_path TEXT NOT NULL,
    notes_path  TEXT NOT NULL DEFAULT '',
    author      TEXT NOT NULL,
    summary     TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    UNIQUE (video_id, n)
);

CREATE TABLE IF NOT EXISTS renders (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    video_id    INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    version     INTEGER NOT NULL,
    kind        TEXT NOT NULL,
    mp4_path    TEXT NOT NULL DEFAULT '',
    report_path TEXT NOT NULL DEFAULT '',
    log_path    TEXT NOT NULL DEFAULT '',
    state       TEXT NOT NULL,
    error       TEXT NOT NULL DEFAULT '',
    seconds     REAL NOT NULL DEFAULT 0,
    started_at  TEXT NOT NULL,
    finished_at TEXT
);

CREATE TABLE IF NOT EXISTS jobs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    video_id   INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL,
    state      TEXT NOT NULL,
    error      TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
