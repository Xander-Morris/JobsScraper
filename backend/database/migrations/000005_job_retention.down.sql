ALTER TABLE job_tags DROP CONSTRAINT IF EXISTS job_tags_job_id_fkey;
ALTER TABLE job_tags ADD CONSTRAINT job_tags_job_id_fkey FOREIGN KEY (job_id) REFERENCES jobs(id);

ALTER TABLE jobs DROP COLUMN IF EXISTS first_seen_at;
