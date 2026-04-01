ALTER TABLE users ADD COLUMN onboarding_complete INTEGER NOT NULL DEFAULT 0;
UPDATE users SET onboarding_complete = 1 WHERE display_name != '' AND display_name IS NOT NULL;
