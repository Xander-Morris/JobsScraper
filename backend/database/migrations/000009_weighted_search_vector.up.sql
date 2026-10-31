-- Weighted search vector: A = title unstemmed, B = title stemmed, C = description unstemmed,
-- D = description stemmed. Unstemmed lexemes let "intern" skip "internal", which the english
-- stemmer folds together. Drop and re-add since SET EXPRESSION needs Postgres 17.
ALTER TABLE jobs DROP COLUMN IF EXISTS search_vector;

ALTER TABLE jobs ADD COLUMN search_vector tsvector GENERATED ALWAYS AS (
	setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
	setweight(to_tsvector('english', coalesce(title, '')), 'B') ||
	setweight(to_tsvector('simple', coalesce(description, '')), 'C') ||
	setweight(to_tsvector('english', coalesce(description, '')), 'D')
) STORED;

CREATE INDEX IF NOT EXISTS idx_jobs_search_vector ON jobs USING GIN(search_vector);
