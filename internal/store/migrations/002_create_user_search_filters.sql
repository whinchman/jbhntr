CREATE TABLE IF NOT EXISTS user_search_filters (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users(id),
    keywords   TEXT NOT NULL DEFAULT '',
    location   TEXT NOT NULL DEFAULT '',
    min_salary INTEGER NOT NULL DEFAULT 0,
    max_salary INTEGER NOT NULL DEFAULT 0,
    title      TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
CREATE INDEX IF NOT EXISTS idx_user_filters_user ON user_search_filters(user_id);
