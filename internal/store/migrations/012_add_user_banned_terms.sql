CREATE TABLE IF NOT EXISTS user_banned_terms (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    term       TEXT   NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, term)
);
CREATE INDEX IF NOT EXISTS idx_user_banned_terms_user ON user_banned_terms(user_id);
