-- Adds vector embeddings for semantic resume-to-job matching, replacing the
-- keyword-only ts_rank scoring used for the "Good fit" badge and the digest
-- email (typed keyword search in the jobs list is untouched by this).

CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE jobs ADD COLUMN IF NOT EXISTS embedding vector(768);

-- HNSW over IVFFlat since it needs no lists/ANALYZE tuning as the table grows.
CREATE INDEX IF NOT EXISTS idx_jobs_embedding ON jobs USING hnsw (embedding vector_cosine_ops);

ALTER TABLE profile_resume_extractions ADD COLUMN IF NOT EXISTS embedding vector(768);
