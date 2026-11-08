-- Descriptions are fetched live from the posting; only the search index built from them is kept.
-- Writes now fill search_vector themselves, with the same weights as 000009.
ALTER TABLE jobs ALTER COLUMN search_vector DROP EXPRESSION;

-- Where the description lives when it isn't the posting page itself, e.g. Greenhouse's per-job API.
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS detail_url TEXT;

ALTER TABLE jobs DROP COLUMN IF EXISTS description;
