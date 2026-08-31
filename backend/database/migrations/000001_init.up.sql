-- Baseline migration mirroring the schema previously created ad hoc by
-- database.CreateTables() (CREATE TABLE IF NOT EXISTS on every boot). Written
-- IF NOT EXISTS throughout so it applies cleanly both to a fresh database and
-- to an existing one that already has this exact schema.

CREATE TABLE IF NOT EXISTS jobs (
	id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	title TEXT NOT NULL,
	company TEXT NOT NULL,
	location TEXT,
	workplace_type INTEGER NOT NULL DEFAULT 0,
	salary_min INTEGER,
	salary_max INTEGER,
	posted_at TIMESTAMPTZ,
	url TEXT NOT NULL,
	description TEXT,
	search_vector tsvector GENERATED ALWAYS AS (to_tsvector('english', coalesce(title, '') || ' ' || coalesce(description, ''))) STORED
);

-- Carried over from the old ad-hoc migration list — guards against an
-- existing database where posted_at predates the TIMESTAMPTZ column type.
DO $$
BEGIN
	IF (SELECT data_type FROM information_schema.columns
		WHERE table_name = 'jobs' AND column_name = 'posted_at') = 'text' THEN
		ALTER TABLE jobs ALTER COLUMN posted_at TYPE TIMESTAMPTZ USING NULLIF(posted_at, '')::timestamptz;
	END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS idx_job_url ON jobs(url);
CREATE INDEX IF NOT EXISTS idx_workplace_type ON jobs(workplace_type);
CREATE INDEX IF NOT EXISTS idx_salary_min ON jobs(salary_min);
CREATE INDEX IF NOT EXISTS idx_salary_max ON jobs(salary_max);
CREATE INDEX IF NOT EXISTS idx_jobs_search_vector ON jobs USING GIN(search_vector);
CREATE INDEX IF NOT EXISTS idx_jobs_posted_at ON jobs(posted_at DESC);

CREATE TABLE IF NOT EXISTS tags (
	id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	tag TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tag ON tags(tag);

CREATE TABLE IF NOT EXISTS job_tags (
	job_id INTEGER REFERENCES jobs(id),
	tag_id INTEGER REFERENCES tags(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_job_tag_pair ON job_tags(job_id, tag_id);

CREATE TABLE IF NOT EXISTS profiles (
	id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	email TEXT NOT NULL,
	password TEXT NOT NULL,
	name TEXT,
	address TEXT,
	linked_in TEXT,
	github TEXT,
	portfolio TEXT,
	email_notifications_enabled BOOLEAN NOT NULL DEFAULT FALSE,
	last_digest_sent_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_email ON profiles(email);

CREATE TABLE IF NOT EXISTS profile_refresh_tokens (
	token_hash TEXT PRIMARY KEY,
	profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
	expires_at TIMESTAMPTZ NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_profile_refresh_tokens_expires_at ON profile_refresh_tokens(expires_at);

CREATE TABLE IF NOT EXISTS profiles_education (
	id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	profile_id INTEGER REFERENCES profiles(id),
	school_name TEXT NOT NULL,
	major TEXT NOT NULL,
	degree TEXT NOT NULL,
	gpa DECIMAL(3,2),
	start_date DATE,
	end_date DATE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_profiles_education_unique ON profiles_education(profile_id, school_name, major, degree);

CREATE TABLE IF NOT EXISTS profiles_work_experience (
	id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	profile_id INTEGER REFERENCES profiles(id),
	company TEXT NOT NULL,
	job_title TEXT NOT NULL,
	job_type INTEGER NOT NULL DEFAULT 0,
	location TEXT,
	start_date DATE,
	end_date DATE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_profiles_work_experience_unique ON profiles_work_experience(profile_id, company, job_title, start_date);

CREATE TABLE IF NOT EXISTS profiles_work_experience_bullets (
	id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	work_experience_id INTEGER REFERENCES profiles_work_experience(id),
	bullet TEXT NOT NULL,
	position INTEGER NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_profiles_work_experience_bullets_unique ON profiles_work_experience_bullets(work_experience_id, bullet);

CREATE TABLE IF NOT EXISTS profiles_skills (
	id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	profile_id INTEGER REFERENCES profiles(id),
	skill TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_profiles_skills_unique ON profiles_skills(profile_id, skill);

CREATE TABLE IF NOT EXISTS profile_resumes (
	id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
	file_name TEXT NOT NULL,
	content_type TEXT NOT NULL,
	file_size INTEGER NOT NULL,
	content BYTEA NOT NULL,
	is_active BOOLEAN NOT NULL DEFAULT FALSE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_profile_resumes_profile_id ON profile_resumes(profile_id);

CREATE TABLE IF NOT EXISTS profile_resume_extractions (
	id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	resume_id INTEGER NOT NULL UNIQUE REFERENCES profile_resumes(id) ON DELETE CASCADE,
	status TEXT NOT NULL DEFAULT 'pending',
	full_name TEXT,
	email TEXT,
	phone TEXT,
	linked_in TEXT,
	github TEXT,
	portfolio TEXT,
	summary TEXT,
	skills JSONB,
	education JSONB,
	work_experience JSONB,
	projects JSONB,
	error TEXT,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS profile_job_applications (
	id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
	job_id INTEGER NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_profile_job_applications_unique ON profile_job_applications(profile_id, job_id);
CREATE INDEX IF NOT EXISTS idx_profile_job_applications_profile_id ON profile_job_applications(profile_id);
