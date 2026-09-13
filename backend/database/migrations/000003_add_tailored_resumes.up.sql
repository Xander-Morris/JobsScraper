CREATE TABLE IF NOT EXISTS profile_tailored_resumes (
	id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
	job_id INTEGER NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
	-- SET NULL so deleting the source resume keeps edited documents (they show as stale)
	resume_id INTEGER REFERENCES profile_resumes(id) ON DELETE SET NULL,
	source_updated_at TIMESTAMPTZ NOT NULL,
	content JSONB NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_profile_tailored_resumes_unique ON profile_tailored_resumes(profile_id, job_id);
