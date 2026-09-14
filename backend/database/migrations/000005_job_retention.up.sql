-- Age fallback for sources that don't give a post date.
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Undated jobs used to store Go's zero time; NULL means unknown now.
UPDATE jobs SET posted_at = NULL WHERE posted_at < '1971-01-01';

-- Lets expired jobs be deleted without clearing their tags first.
ALTER TABLE job_tags DROP CONSTRAINT IF EXISTS job_tags_job_id_fkey;
ALTER TABLE job_tags ADD CONSTRAINT job_tags_job_id_fkey FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE;
