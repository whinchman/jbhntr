-- Migration 015: backfill provider_id for email-auth users.
-- Previously CreateUserWithPassword inserted provider_id='' for all email
-- users, causing the UNIQUE(provider, provider_id) constraint to fire on the
-- second signup and incorrectly surface as "email already taken".
-- Set provider_id = email for all affected rows so the data is consistent
-- with the fixed insertion logic.
UPDATE users
SET provider_id = email
WHERE provider = 'email'
  AND provider_id = ''
  AND email != '';
