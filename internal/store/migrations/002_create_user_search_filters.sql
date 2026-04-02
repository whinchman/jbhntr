CREATE TABLE IF NOT EXISTS user_search_filters (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id),
    keywords   TEXT NOT NULL DEFAULT '',
    location   TEXT NOT NULL DEFAULT '',
    min_salary INTEGER NOT NULL DEFAULT 0,
    max_salary INTEGER NOT NULL DEFAULT 0,
    title      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_user_filters_user ON user_search_filters(user_id);
