DROP INDEX IF EXISTS idx_jobs_full_time_age;
DROP INDEX IF EXISTS idx_jobs_part_time_age;
DROP INDEX IF EXISTS idx_jobs_intern_age;

ALTER TABLE jobs
	DROP COLUMN IF EXISTS is_intern,
	DROP COLUMN IF EXISTS is_part_time,
	DROP COLUMN IF EXISTS is_full_time;

DROP INDEX IF EXISTS idx_job_tags_tag_job;
DROP INDEX IF EXISTS idx_profile_resumes_active;
DROP INDEX IF EXISTS idx_jobs_age;
