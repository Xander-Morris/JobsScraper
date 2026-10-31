ALTER TABLE jobs DROP COLUMN IF EXISTS search_vector;

ALTER TABLE jobs ADD COLUMN search_vector tsvector GENERATED ALWAYS AS (
	to_tsvector('english', coalesce(title, '') || ' ' || coalesce(description, ''))
) STORED;

CREATE INDEX IF NOT EXISTS idx_jobs_search_vector ON jobs USING GIN(search_vector);
