package database

import "encoding/json"

type Schema map[string]TableDefinition

type TableDefinition struct {
	Columns         []map[string]string
	Indexes         []string
	InsertStatement string
}

type JobType int

const (
	JobTypeUnknown JobType = iota
	JobTypeContract
	JobTypeInternship
	JobTypePartTime
	JobTypeFullTime
)

func ParseJobType(s string) (JobType, bool) {
	switch s {
	case "":
		return JobTypeUnknown, true
	case "contract":
		return JobTypeContract, true
	case "internship":
		return JobTypeInternship, true
	case "part_time":
		return JobTypePartTime, true
	case "full_time":
		return JobTypeFullTime, true
	default:
		return JobTypeUnknown, false
	}
}

func (j JobType) String() string {
	switch j {
	case JobTypeContract:
		return "contract"
	case JobTypeInternship:
		return "internship"
	case JobTypePartTime:
		return "part_time"
	case JobTypeFullTime:
		return "full_time"
	default:
		return "unknown"
	}
}

func (j JobType) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.String())
}

func (j *JobType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	parsed, _ := ParseJobType(s)
	*j = parsed

	return nil
}

/**
	tableCreationOrder lists table names in dependency order (referenced tables
	before the tables that REFERENCE them). Go randomizes map iteration order,
	so CreateTables must not range over the tables map directly when creating
	tables with foreign keys.
**/
var tableCreationOrder = []string{
	"jobs",
	"tags",
	"job_tags",
	"profiles",
	"profiles_education",
	"profiles_work_experience",
	"profiles_work_experience_bullets",
	"profiles_skills",
}

var tables = Schema{
	"jobs": TableDefinition{
		Columns: []map[string]string{
			{"id": "INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY"},
			{"title": "TEXT NOT NULL"},
			{"company": "TEXT NOT NULL"},
			{"location": "TEXT"},
			{"workplace_type": "INTEGER NOT NULL DEFAULT 0"},
			{"salary_min": "INTEGER"},
			{"salary_max": "INTEGER"},
			{"posted_at": "TEXT"},
			{"url": "TEXT NOT NULL"},
			{"description": "TEXT"},
			{"search_vector": "tsvector GENERATED ALWAYS AS (to_tsvector('english', coalesce(title, '') || ' ' || coalesce(description, ''))) STORED"},
		},
		Indexes: []string{
			"CREATE UNIQUE INDEX IF NOT EXISTS idx_job_url ON jobs(url);",
			"CREATE INDEX IF NOT EXISTS idx_workplace_type ON jobs(workplace_type)",
			"CREATE INDEX IF NOT EXISTS idx_salary_min ON jobs(salary_min)",
			"CREATE INDEX IF NOT EXISTS idx_salary_max ON jobs(salary_max)",
			"CREATE INDEX IF NOT EXISTS idx_jobs_search_vector ON jobs USING GIN(search_vector);",
		},
		InsertStatement: `INSERT INTO jobs (title, company, location, workplace_type, salary_min, salary_max, posted_at, url, description)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT(url) DO UPDATE SET
				title = excluded.title,
				company = excluded.company,
				location = excluded.location,
				workplace_type = excluded.workplace_type,
				salary_min = excluded.salary_min,
				salary_max = excluded.salary_max,
				posted_at = excluded.posted_at,
				description = excluded.description
			RETURNING id;`,
	},
	"tags": TableDefinition{
		Columns: []map[string]string{
			{"id": "INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY"},
			{"tag": "TEXT NOT NULL"},
		},
		Indexes: []string{
			"CREATE UNIQUE INDEX IF NOT EXISTS idx_tag ON tags(tag);",
		},
		InsertStatement: `INSERT INTO tags (tag) VALUES ($1) ON CONFLICT(tag) DO UPDATE SET tag=excluded.tag RETURNING id;`,
	},
	"job_tags": TableDefinition{
		Columns: []map[string]string{
			{"job_id": "INTEGER REFERENCES jobs(id)"},
			{"tag_id": "INTEGER REFERENCES tags(id)"},
		},
		Indexes: []string{
			"CREATE UNIQUE INDEX IF NOT EXISTS idx_job_tag_pair ON job_tags(job_id, tag_id);",
		},
		InsertStatement: `INSERT INTO job_tags (job_id, tag_id) VALUES ($1, $2) ON CONFLICT (job_id, tag_id) DO NOTHING;`,
	},
	"profiles": TableDefinition{
		Columns: []map[string]string{
			// email and password required at first, but rest can be filled in as they want to
			{"id": "INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY"},
			{"email": "TEXT NOT NULL"},
			{"password": "TEXT NOT NULL"},

			// to be filled in dynamically when they decide to on profile view
			{"name": "TEXT"},
			{"address": "TEXT"},
			{"linked_in": "TEXT"},
			{"github": "TEXT"},
			{"portfolio": "TEXT"},
		},
		Indexes: []string{
			"CREATE UNIQUE INDEX IF NOT EXISTS idx_email ON profiles(email);",
		},
		InsertStatement: `INSERT INTO profiles (email, password) VALUES ($1, $2) ON CONFLICT (email) DO NOTHING RETURNING id;`,
	},
	"profiles_education": TableDefinition{
		Columns: []map[string]string{
			{"id": "INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY"},
			{"profile_id": "INTEGER REFERENCES profiles(id)"},
			{"school_name": "TEXT NOT NULL"},
			{"major": "TEXT NOT NULL"},
			{"degree": "TEXT NOT NULL"},
			{"gpa": "DECIMAL(3,2)"},
			{"start_date": "DATE"},
			{"end_date": "DATE"},
		},
		Indexes: []string{
			"CREATE UNIQUE INDEX IF NOT EXISTS idx_profiles_education_unique ON profiles_education(profile_id, school_name, major, degree);",
		},
		InsertStatement: `INSERT INTO profiles_education (profile_id, school_name, major, degree, gpa, start_date, end_date)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (profile_id, school_name, major, degree) DO UPDATE SET
				gpa = excluded.gpa,
				start_date = excluded.start_date,
				end_date = excluded.end_date
			RETURNING id;`,
	},
	"profiles_work_experience": TableDefinition{
		Columns: []map[string]string{
			{"id": "INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY"},
			{"profile_id": "INTEGER REFERENCES profiles(id)"},
			{"company": "TEXT NOT NULL"},
			{"job_title": "TEXT NOT NULL"},
			{"job_type": "INTEGER NOT NULL DEFAULT 0"},
			{"location": "TEXT"},
			{"start_date": "DATE"},
			{"end_date": "DATE"},
		},
		Indexes: []string{
			"CREATE UNIQUE INDEX IF NOT EXISTS idx_profiles_work_experience_unique ON profiles_work_experience(profile_id, company, job_title, start_date);",
		},
		InsertStatement: `INSERT INTO profiles_work_experience (profile_id, company, job_title, job_type, location, start_date, end_date)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (profile_id, company, job_title, start_date) DO UPDATE SET
				job_type = excluded.job_type,
				location = excluded.location,
				end_date = excluded.end_date
			RETURNING id;`,
	},
	"profiles_work_experience_bullets": TableDefinition{
		Columns: []map[string]string{
			{"id": "INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY"},
			{"work_experience_id": "INTEGER REFERENCES profiles_work_experience(id)"},
			{"bullet": "TEXT NOT NULL"},
			{"position": "INTEGER NOT NULL DEFAULT 0"},
		},
		Indexes: []string{
			"CREATE UNIQUE INDEX IF NOT EXISTS idx_profiles_work_experience_bullets_unique ON profiles_work_experience_bullets(work_experience_id, bullet);",
		},
		InsertStatement: `INSERT INTO profiles_work_experience_bullets (work_experience_id, bullet, position)
			VALUES ($1, $2, $3)
			ON CONFLICT (work_experience_id, bullet) DO UPDATE SET
				position = excluded.position
			RETURNING id;`,
	},
	"profiles_skills": TableDefinition{
		Columns: []map[string]string{
			{"id": "INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY"},
			{"profile_id": "INTEGER REFERENCES profiles(id)"},
			{"skill": "TEXT NOT NULL"},
		},
		Indexes: []string{
			"CREATE UNIQUE INDEX IF NOT EXISTS idx_profiles_skills_unique ON profiles_skills(profile_id, skill);",
		},
		InsertStatement: `INSERT INTO profiles_skills (profile_id, skill) VALUES ($1, $2) ON CONFLICT (profile_id, skill) DO UPDATE SET skill = excluded.skill RETURNING id;`,
	},
}
