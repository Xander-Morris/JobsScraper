package database

type Schema map[string]TableDefinition

type TableDefinition struct {
	Columns         []map[string]string
	Indexes         []string
	InsertStatement string
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
}
