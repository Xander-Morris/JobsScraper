package database

import (
	"database/sql"
	"fmt"
)

type Schema map[string]TableDefinition

type TableDefinition struct {
	Columns         []map[string]string
	Indexes         []string
	InsertStatement string
}

var tables = Schema{
	"jobs": TableDefinition{
		Columns: []map[string]string{
			{"id": "INTEGER PRIMARY KEY AUTOINCREMENT"},
			{"title": "TEXT NOT NULL"},
			{"company": "TEXT NOT NULL"},
			{"location": "TEXT"},
			{"workplace_type": "INTEGER NOT NULL DEFAULT 0"},
			{"salary_min": "INTEGER"},
			{"salary_max": "INTEGER"},
			{"posted_at": "TEXT"},
			{"url": "TEXT NOT NULL"},
			{"description": "TEXT"},
		},
		Indexes: []string{
			"CREATE UNIQUE INDEX IF NOT EXISTS idx_job_url ON jobs(url);",
			"CREATE INDEX IF NOT EXISTS idx_workplace_type ON jobs(workplace_type)",
			"CREATE INDEX IF NOT EXISTS idx_salary_min ON jobs(salary_min)",
			"CREATE INDEX IF NOT EXISTS idx_salary_max ON jobs(salary_max)",
		},
		InsertStatement: `INSERT INTO jobs (title, company, location, workplace_type, salary_min, salary_max, posted_at, url, description)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			{"id": "INTEGER PRIMARY KEY AUTOINCREMENT"},
			{"tag": "TEXT NOT NULL"},
		},
		Indexes: []string{
			"CREATE UNIQUE INDEX IF NOT EXISTS idx_tag ON tags(tag);",
		},
		InsertStatement: `INSERT INTO tags (tag) VALUES (?) ON CONFLICT(tag) DO UPDATE SET tag=excluded.tag RETURNING id;`,
	},
	"job_tags": TableDefinition{
		Columns: []map[string]string{
			{"job_id": "INTEGER REFERENCES jobs(id)"},
			{"tag_id": "INTEGER REFERENCES tags(id)"},
		},
		Indexes: []string{
			"CREATE UNIQUE INDEX IF NOT EXISTS idx_job_tag_pair ON job_tags(job_id, tag_id);",
		},
		InsertStatement: `INSERT OR IGNORE INTO job_tags (job_id, tag_id) VALUES (?, ?);`,
	},
}

const jobsFTSTable = `CREATE VIRTUAL TABLE IF NOT EXISTS jobs_fts USING fts5(
	title, description, content='jobs', content_rowid='id'
);`

var jobsFTSTriggers = []string{
	`CREATE TRIGGER IF NOT EXISTS jobs_fts_ai AFTER INSERT ON jobs BEGIN
		INSERT INTO jobs_fts(rowid, title, description) VALUES (new.id, new.title, new.description);
	END;`,
	`CREATE TRIGGER IF NOT EXISTS jobs_fts_ad AFTER DELETE ON jobs BEGIN
		INSERT INTO jobs_fts(jobs_fts, rowid, title, description) VALUES ('delete', old.id, old.title, old.description);
	END;`,
	`CREATE TRIGGER IF NOT EXISTS jobs_fts_au AFTER UPDATE ON jobs BEGIN
		INSERT INTO jobs_fts(jobs_fts, rowid, title, description) VALUES ('delete', old.id, old.title, old.description);
		INSERT INTO jobs_fts(rowid, title, description) VALUES (new.id, new.title, new.description);
	END;`,
}

const backfillJobsFTS = `INSERT INTO jobs_fts(rowid, title, description)
	SELECT id, title, description FROM jobs
	WHERE id NOT IN (SELECT rowid FROM jobs_fts);`

func createFullTextSearch(db *sql.DB) error {
	if _, err := db.Exec(jobsFTSTable); err != nil {
		return fmt.Errorf("create jobs_fts table: %w", err)
	}

	for _, trigger := range jobsFTSTriggers {
		if _, err := db.Exec(trigger); err != nil {
			return fmt.Errorf("create fts sync trigger: %w", err)
		}
	}

	if _, err := db.Exec(backfillJobsFTS); err != nil {
		return fmt.Errorf("backfill jobs_fts: %w", err)
	}

	return nil
}
