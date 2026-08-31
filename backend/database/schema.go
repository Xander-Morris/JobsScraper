package database

import "encoding/json"

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
	case "", "unknown":
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

// insertStatements holds the upsert SQL for tables written outside their own
// dedicated files. Table DDL itself lives in migrations/ (see migrate.go).
var insertStatements = map[string]string{
	"jobs": `INSERT INTO jobs (title, company, location, workplace_type, salary_min, salary_max, posted_at, url, description)
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

	"tags": `INSERT INTO tags (tag) VALUES ($1) ON CONFLICT(tag) DO UPDATE SET tag=excluded.tag RETURNING id;`,

	"job_tags": `INSERT INTO job_tags (job_id, tag_id) VALUES ($1, $2) ON CONFLICT (job_id, tag_id) DO NOTHING;`,

	"profiles": `INSERT INTO profiles (email, password) VALUES ($1, $2) ON CONFLICT (email) DO NOTHING RETURNING id;`,

	"profiles_education": `INSERT INTO profiles_education (profile_id, school_name, major, degree, gpa, start_date, end_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (profile_id, school_name, major, degree) DO UPDATE SET
			gpa = excluded.gpa,
			start_date = excluded.start_date,
			end_date = excluded.end_date
		RETURNING id;`,

	"profiles_work_experience": `INSERT INTO profiles_work_experience (profile_id, company, job_title, job_type, location, start_date, end_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (profile_id, company, job_title, start_date) DO UPDATE SET
			job_type = excluded.job_type,
			location = excluded.location,
			end_date = excluded.end_date
		RETURNING id;`,

	"profiles_work_experience_bullets": `INSERT INTO profiles_work_experience_bullets (work_experience_id, bullet, position)
		VALUES ($1, $2, $3)
		ON CONFLICT (work_experience_id, bullet) DO UPDATE SET
			position = excluded.position
		RETURNING id;`,

	"profiles_skills": `INSERT INTO profiles_skills (profile_id, skill) VALUES ($1, $2) ON CONFLICT (profile_id, skill) DO UPDATE SET skill = excluded.skill RETURNING id;`,

	"profile_job_applications": `INSERT INTO profile_job_applications (profile_id, job_id) VALUES ($1, $2)
		ON CONFLICT (profile_id, job_id) DO UPDATE SET applied_at = NOW()
		RETURNING id;`,
}
