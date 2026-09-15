-- Every search filters on job age, and date sort orders by it (id breaks ties).
CREATE INDEX IF NOT EXISTS idx_jobs_age ON jobs ((COALESCE(posted_at, first_seen_at)) DESC NULLS LAST, id DESC);

-- Signed-in searches look up the active resume first.
CREATE INDEX IF NOT EXISTS idx_profile_resumes_active ON profile_resumes(profile_id) WHERE is_active;

-- Tag filter goes tag -> jobs; idx_job_tag_pair only covers job -> tags.
CREATE INDEX IF NOT EXISTS idx_job_tags_tag_job ON job_tags(tag_id, job_id);

-- Job type computed at write time instead of a regex per row per search.
ALTER TABLE jobs
	ADD COLUMN IF NOT EXISTS is_intern BOOLEAN NOT NULL DEFAULT FALSE,
	ADD COLUMN IF NOT EXISTS is_part_time BOOLEAN NOT NULL DEFAULT FALSE,
	ADD COLUMN IF NOT EXISTS is_full_time BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE jobs j SET
	is_intern = j.title ~* '\mintern(s|ships?)?\M' OR EXISTS (SELECT 1 FROM job_tags jt JOIN tags t ON t.id = jt.tag_id
		WHERE jt.job_id = j.id AND t.tag ~* '\mintern(s|ships?)?\M'),
	is_part_time = j.title ~* '\mpart[-_ ]?time\M' OR EXISTS (SELECT 1 FROM job_tags jt JOIN tags t ON t.id = jt.tag_id
		WHERE jt.job_id = j.id AND t.tag ~* '\mpart[-_ ]?time\M'),
	is_full_time = j.title ~* '\mfull[-_ ]?time\M' OR EXISTS (SELECT 1 FROM job_tags jt JOIN tags t ON t.id = jt.tag_id
		WHERE jt.job_id = j.id AND t.tag ~* '\mfull[-_ ]?time\M');

CREATE INDEX IF NOT EXISTS idx_jobs_intern_age ON jobs ((COALESCE(posted_at, first_seen_at)) DESC NULLS LAST, id DESC) WHERE is_intern;
CREATE INDEX IF NOT EXISTS idx_jobs_part_time_age ON jobs ((COALESCE(posted_at, first_seen_at)) DESC NULLS LAST, id DESC) WHERE is_part_time;
CREATE INDEX IF NOT EXISTS idx_jobs_full_time_age ON jobs ((COALESCE(posted_at, first_seen_at)) DESC NULLS LAST, id DESC) WHERE is_full_time;
