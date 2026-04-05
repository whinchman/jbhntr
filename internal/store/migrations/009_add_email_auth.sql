-- Password hash (bcrypt). NULL means the account was created via OAuth only.
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT;

-- Email verification. Default true for existing OAuth users (their email was
-- verified by the provider).
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified INTEGER NOT NULL DEFAULT 1;

-- One-time email verification token and its expiry.
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verify_token TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verify_expires_at TIMESTAMPTZ;

-- Password-reset token and its expiry.
ALTER TABLE users ADD COLUMN IF NOT EXISTS reset_token TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS reset_expires_at TIMESTAMPTZ;

-- Unique index on email for GetUserByEmail lookups.
-- Allow multiple rows with empty email (legacy OAuth rows without email).
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique
    ON users (email)
    WHERE email != '';
