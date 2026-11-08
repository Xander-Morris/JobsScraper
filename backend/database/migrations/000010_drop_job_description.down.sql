-- Dropped descriptions can't come back; they refill on the next scrape.
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE jobs DROP COLUMN IF EXISTS detail_url;

ALTER TABLE jobs DROP COLUMN IF EXISTS search_vector;

ALTER TABLE jobs ADD COLUMN search_vector tsvector GENERATED ALWAYS AS (
	setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
	setweight(to_tsvector('english', coalesce(title, '')), 'B') ||
	setweight(to_tsvector('simple', coalesce(description, '')), 'C') ||
	setweight(to_tsvector('english', coalesce(description, '')), 'D')
) STORED;

CREATE INDEX IF NOT EXISTS idx_jobs_search_vector ON jobs USING GIN(search_vector);
