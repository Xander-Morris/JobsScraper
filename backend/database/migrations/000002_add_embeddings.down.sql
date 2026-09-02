DROP INDEX IF EXISTS idx_jobs_embedding;
ALTER TABLE jobs DROP COLUMN IF EXISTS embedding;
ALTER TABLE profile_resume_extractions DROP COLUMN IF EXISTS embedding;
