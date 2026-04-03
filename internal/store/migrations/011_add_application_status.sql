ALTER TABLE jobs
  ADD COLUMN IF NOT EXISTS application_status TEXT
      CHECK(application_status IN ('applied','interviewing','lost','won')),
  ADD COLUMN IF NOT EXISTS applied_at        TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS interviewing_at   TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS lost_at           TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS won_at            TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_jobs_application_status
    ON jobs(user_id, application_status)
    WHERE application_status IS NOT NULL;
