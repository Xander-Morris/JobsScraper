package database

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"main/internal/faultdb"
	"main/jobs"
	"main/llm"
	"main/utils"

	"github.com/pgvector/pgvector-go"
)

// newFaultDB is newTestDB on a pool whose operations the returned injector can fail.
func newFaultDB(t *testing.T) *faultdb.Injector {
	t.Helper()
	newTestDB(t)

	pool, inj, err := faultdb.Open(utils.Getenv("TEST_DATABASE_CONNECTION"))
	if err != nil {
		t.Fatalf("open fault db: %v", err)
	}

	prev := SetDB(pool)
	t.Cleanup(func() {
		pool.Close()
		SetDB(prev)
	})

	return inj
}

// must unwraps fixture setup; a failure panics, which fails the test.
func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func mustDo(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// seq hands out unique suffixes so repeated setups don't collide.
type seq int

func (s *seq) next() int {
	*s++
	return int(*s)
}

func TestFaultsProfiles(t *testing.T) {
	inj := newFaultDB(t)
	ctx := t.Context()
	pid := newTestProfile(t)
	var n seq

	// Every list GetProfile reads gets a row, so each scan loop runs.
	gpa := 3.5
	must(AddEducation(ctx, pid, &AddEducationRequest{SchoolName: "MIT", Major: "CS", Degree: "BS", GPA: &gpa, StartDate: "2018-09-01", EndDate: "2022-05-01"}))
	must(AddSkill(ctx, pid, &AddSkillRequest{Skill: "Go"}))
	weID := must(AddWorkExperience(ctx, pid, &AddWorkExperienceRequest{Company: "Acme", JobTitle: "Dev", JobType: "full_time", Location: "NYC", StartDate: "2020-01-01", EndDate: "2021-01-01"}))
	must(AddWorkExperienceBullet(ctx, pid, weID, &AddWorkExperienceBulletRequest{Bullet: "Shipped"}))
	must(AddResume(ctx, pid, "cv.pdf", "application/pdf", []byte("x")))
	mustDo(t, UpdateProfile(ctx, pid, &UpdateProfileRequest{EmailNotifications: true}))

	t.Run("GetProfile", func(t *testing.T) {
		faultdb.Sweep(t, inj, nil, func() error { _, err := GetProfile(ctx, pid); return err })
	})
	t.Run("GetProfileByEmail", func(t *testing.T) {
		faultdb.Sweep(t, inj, nil, func() error { _, _, err := GetProfileByEmail(ctx, "test@example.com"); return err })
	})
	t.Run("CreateProfile", func(t *testing.T) {
		faultdb.Sweep(t, inj, nil, func() error {
			_, err := CreateProfile(ctx, &ProfileRequest{Email: fmt.Sprintf("p%d@example.com", n.next()), Password: "securepassword123"})
			return err
		})
	})
	t.Run("UpdateProfile", func(t *testing.T) {
		faultdb.Sweep(t, inj, nil, func() error {
			return UpdateProfile(ctx, pid, &UpdateProfileRequest{Name: "Ada", EmailNotifications: true})
		})
	})
	t.Run("digest", func(t *testing.T) {
		faultdb.Sweep(t, inj, nil, func() error { _, err := ListProfilesForDigest(ctx); return err })
		faultdb.Sweep(t, inj, nil, func() error { return MarkDigestSent(ctx, pid, time.Now()) })
		faultdb.Sweep(t, inj, nil, func() error { return DisableEmailDigest(ctx, pid) })
	})
}

func TestFaultsProfileSections(t *testing.T) {
	inj := newFaultDB(t)
	ctx := t.Context()
	pid := newTestProfile(t)
	var n seq

	edu := func() *AddEducationRequest {
		return &AddEducationRequest{SchoolName: fmt.Sprintf("School %d", n.next()), Major: "CS", Degree: "BS"}
	}
	var eduID int64
	newEdu := func() { eduID = must(AddEducation(ctx, pid, edu())) }

	t.Run("education", func(t *testing.T) {
		faultdb.Sweep(t, inj, nil, func() error { _, err := AddEducation(ctx, pid, edu()); return err })
		faultdb.Sweep(t, inj, newEdu, func() error { return UpdateEducation(ctx, pid, eduID, edu()) })
		faultdb.Sweep(t, inj, newEdu, func() error { return DeleteEducation(ctx, pid, eduID) })
	})

	var skillID int64
	t.Run("skills", func(t *testing.T) {
		faultdb.Sweep(t, inj, nil, func() error {
			_, err := AddSkill(ctx, pid, &AddSkillRequest{Skill: fmt.Sprintf("s%d", n.next())})
			return err
		})
		faultdb.Sweep(t, inj, func() {
			skillID = must(AddSkill(ctx, pid, &AddSkillRequest{Skill: fmt.Sprintf("s%d", n.next())}))
		}, func() error { return DeleteSkill(ctx, pid, skillID) })
	})

	work := func() *AddWorkExperienceRequest {
		return &AddWorkExperienceRequest{Company: fmt.Sprintf("Co %d", n.next()), JobTitle: "Dev", JobType: "full_time", StartDate: "2020-01-01"}
	}
	var weID, bulletID int64
	newWork := func() {
		weID = must(AddWorkExperience(ctx, pid, work()))
		bulletID = must(AddWorkExperienceBullet(ctx, pid, weID, &AddWorkExperienceBulletRequest{Bullet: "b"}))
	}

	t.Run("work experience", func(t *testing.T) {
		faultdb.Sweep(t, inj, nil, func() error { _, err := AddWorkExperience(ctx, pid, work()); return err })
		faultdb.Sweep(t, inj, newWork, func() error { return UpdateWorkExperience(ctx, pid, weID, work()) })
		faultdb.Sweep(t, inj, newWork, func() error { return DeleteWorkExperience(ctx, pid, weID) })
		faultdb.Sweep(t, inj, newWork, func() error {
			_, err := AddWorkExperienceBullet(ctx, pid, weID, &AddWorkExperienceBulletRequest{Bullet: "more"})
			return err
		})
		faultdb.Sweep(t, inj, newWork, func() error {
			return UpdateWorkExperienceBullet(ctx, pid, weID, bulletID, &UpdateWorkExperienceBulletRequest{Bullet: "edited"})
		})
		faultdb.Sweep(t, inj, newWork, func() error { return DeleteWorkExperienceBullet(ctx, pid, weID, bulletID) })
	})
}

func TestFaultsResumes(t *testing.T) {
	inj := newFaultDB(t)
	ctx := t.Context()
	pid := newTestProfile(t)

	var resumeID int64
	newResume := func() {
		resumeID = must(AddResume(ctx, pid, "cv.pdf", "application/pdf", []byte("x")))
		mustDo(t, SaveResumeExtractionResult(ctx, resumeID, &llm.ExtractedResume{
			Skills:         []string{"Go"},
			Education:      []llm.EducationEntry{{SchoolName: "MIT"}},
			WorkExperience: []llm.WorkExperienceEntry{{Company: "Acme"}},
			Projects:       []llm.ProjectEntry{{Name: "Crawler"}},
		}))
	}
	newResume()
	mustDo(t, SetActiveResume(ctx, pid, resumeID))

	faultdb.Sweep(t, inj, nil, func() error { _, err := ListResumes(ctx, pid); return err })
	faultdb.Sweep(t, inj, nil, func() error {
		_, err := AddResume(ctx, pid, "b.pdf", "application/pdf", []byte("y"))
		return err
	})
	faultdb.Sweep(t, inj, newResume, func() error { return SetActiveResume(ctx, pid, resumeID) })
	faultdb.Sweep(t, inj, newResume, func() error {
		return ReplaceResume(ctx, pid, resumeID, "c.pdf", "application/pdf", []byte("z"))
	})
	faultdb.Sweep(t, inj, newResume, func() error { return RenameResume(ctx, pid, resumeID, "renamed.pdf") })
	faultdb.Sweep(t, inj, newResume, func() error { _, _, err := GetResume(ctx, pid, resumeID); return err })
	faultdb.Sweep(t, inj, newResume, func() error { return DeleteResume(ctx, pid, resumeID) })
	faultdb.Sweep(t, inj, newResume, func() error { return UpsertResumeExtractionPending(ctx, resumeID) })
	faultdb.Sweep(t, inj, newResume, func() error {
		return SaveResumeExtractionResult(ctx, resumeID, &llm.ExtractedResume{FullName: "Ada"})
	})
	faultdb.Sweep(t, inj, newResume, func() error { return SaveResumeExtractionFailure(ctx, resumeID, "failed", "boom") })
	faultdb.Sweep(t, inj, newResume, func() error { return SaveResumeEmbedding(ctx, resumeID, make([]float32, 768)) })
	faultdb.Sweep(t, inj, newResume, func() error { _, err := GetResumeExtraction(ctx, pid, resumeID); return err })
	faultdb.Sweep(t, inj, nil, func() error { _, _, err := GetActiveResumeExtraction(ctx, pid); return err })
}

func TestFaultsSessions(t *testing.T) {
	inj := newFaultDB(t)
	ctx := t.Context()
	pid := newTestProfile(t)

	var token, resetToken string
	newToken := func() {
		token = NewRefreshToken()
		mustDo(t, StoreRefreshToken(ctx, pid, token, time.Now().Add(time.Hour), false))
	}

	faultdb.Sweep(t, inj, nil, func() error {
		return StoreRefreshToken(ctx, pid, NewRefreshToken(), time.Now().Add(time.Hour), true)
	})
	faultdb.Sweep(t, inj, newToken, func() error {
		_, err := RotateRefreshToken(ctx, token, NewRefreshToken())
		return err
	})
	faultdb.Sweep(t, inj, newToken, func() error { return DeleteRefreshToken(ctx, token) })
	faultdb.Sweep(t, inj, nil, func() error { _, err := CreatePasswordResetToken(ctx, pid); return err })
	faultdb.Sweep(t, inj, func() { resetToken = must(CreatePasswordResetToken(ctx, pid)) }, func() error {
		_, err := ResetPassword(ctx, resetToken, "anothersecurepassword")
		return err
	})
}

func TestFaultsJobs(t *testing.T) {
	inj := newFaultDB(t)
	ctx := t.Context()
	pid := newTestProfile(t)
	fakeJina(t)
	var n seq

	salary := 100000
	batch := func() []jobs.Job {
		i := n.next()
		return []jobs.Job{
			{Title: "Go Dev", Company: "Acme", URL: fmt.Sprintf("https://example.com/f%d", i), Tags: []string{"go", fmt.Sprintf("t%d", i)}, SalaryMin: &salary, PostedAt: time.Now(), Description: "Go APIs"},
			{Title: "Old", Company: "Acme", URL: fmt.Sprintf("https://example.com/old%d", i), Tags: []string{"old"}, PostedAt: time.Now().Add(-JobMaxAge - time.Hour), Description: "Legacy"},
		}
	}
	seedJobs(t, batch()...)
	jobID := jobIDByURL(t, "https://example.com/f1")
	emb := pgvector.NewVector(unitVector(0))
	posted := time.Now().Add(-time.Hour * 24 * 365)

	faultdb.Sweep(t, inj, nil, func() error { return WriteJobsToDatabase(ctx, batch()) })
	faultdb.Sweep(t, inj, nil, func() error {
		_, err := KnownJobURLs(ctx, []string{"https://example.com/f1"})
		return err
	})
	// Offset past the end also runs the fallback count query.
	for _, offset := range []int{0, 1000} {
		faultdb.Sweep(t, inj, nil, func() error {
			_, err := SearchForJobs(ctx, &JobSearchParams{SearchQuery: "go", ResumeEmbedding: &emb, ProfileID: pid, PostedAfter: &posted, MaxSalary: 500000, Tags: []string{"go"}, Offset: offset})
			return err
		})
	}
	faultdb.Sweep(t, inj, nil, func() error {
		_, err := GetJobByID(ctx, jobID, JobDetailParams{ResumeEmbedding: &emb, ProfileID: pid})
		return err
	})
	faultdb.Sweep(t, inj, nil, func() error { _, err := FetchAllUniqueTags(ctx); return err })
	faultdb.Sweep(t, inj, nil, func() error { return MarkJobApplied(ctx, pid, jobID) })
	faultdb.Sweep(t, inj, nil, func() error { return UnmarkJobApplied(ctx, pid, jobID) })
	faultdb.Sweep(t, inj, func() { seedJobs(t, batch()...) }, func() error { _, err := DeleteExpiredJobs(ctx); return err })
	faultdb.Sweep(t, inj, func() {
		_, err := DB().ExecContext(ctx, "UPDATE jobs SET embedding = NULL")
		mustDo(t, err)
	}, func() error { return EmbedPendingJobs(ctx) })
}

func TestFaultsTailoredAndTrivia(t *testing.T) {
	inj := newFaultDB(t)
	ctx := t.Context()
	pid := newTestProfile(t)
	seedJobs(t, jobs.Job{Title: "Go Dev", Company: "Acme", URL: "https://example.com/t"})
	jobID := jobIDByURL(t, "https://example.com/t")
	resumeID := must(AddResume(ctx, pid, "cv.pdf", "application/pdf", []byte("x")))
	content := llm.TailoredResume{FullName: "Ada"}
	mustDo(t, UpsertTailoredResume(ctx, pid, jobID, resumeID, time.Now(), content))

	faultdb.Sweep(t, inj, nil, func() error { _, err := GetTailoredResume(ctx, pid, jobID); return err })
	faultdb.Sweep(t, inj, nil, func() error { return UpsertTailoredResume(ctx, pid, jobID, resumeID, time.Now(), content) })
	faultdb.Sweep(t, inj, nil, func() error { return UpdateTailoredResumeContent(ctx, pid, jobID, content) })

	questions := []llm.TriviaQuestion{{Question: "q", Options: []string{"a", "b"}, CorrectIndex: 1}}
	var quizID int64
	newQuiz := func() { quizID = must(CreateTriviaQuiz(ctx, pid, "prompt", "Go", "easy", questions)) }
	newQuiz()

	faultdb.Sweep(t, inj, nil, func() error {
		_, err := CreateTriviaQuiz(ctx, pid, "prompt", "Go", "easy", questions)
		return err
	})
	faultdb.Sweep(t, inj, nil, func() error { _, err := ListTriviaQuizzes(ctx, pid); return err })
	faultdb.Sweep(t, inj, nil, func() error { _, err := GetTriviaQuiz(ctx, pid, quizID); return err })
	faultdb.Sweep(t, inj, newQuiz, func() error { return SaveTriviaAnswers(ctx, pid, quizID, []int{1}) })
	faultdb.Sweep(t, inj, nil, func() error { _, err := RetakeTriviaQuiz(ctx, pid, quizID); return err })
	faultdb.Sweep(t, inj, newQuiz, func() error { return DeleteTriviaQuiz(ctx, pid, quizID) })
}

func TestFaultsCreateTables(t *testing.T) {
	inj := newFaultDB(t)

	admin, err := sql.Open("pgx", utils.Getenv("TEST_DATABASE_CONNECTION"))
	mustDo(t, err)
	t.Cleanup(func() { admin.Close() })

	// A fault on unlock leaves migrate's advisory lock held; end that session so the next attempt can lock.
	reset := func() {
		migrated = false
		_, err := admin.Exec(`SELECT pg_terminate_backend(pid) FROM pg_locks WHERE locktype = 'advisory' AND pid <> pg_backend_pid()`)
		mustDo(t, err)
	}

	// No assertion: migrate drops its own unlock error, so one fault is ignored by design.
	faultdb.SweepEach(t, inj, reset, func() { _ = CreateTables() }, func(*testing.T) {})
	reset()
	mustDo(t, CreateTables())
}

func TestCreateTablesMissingMigrations(t *testing.T) {
	newTestDB(t)
	migrationsDir = "missing"
	t.Cleanup(func() { migrationsDir = "migrations" })
	migrated = false

	if err := CreateTables(); err == nil {
		t.Fatal("CreateTables with no migrations dir succeeded")
	}
}

func TestCreateTablesFailsBeforeWritingJobs(t *testing.T) {
	newTestDB(t)
	migrationsDir = "missing"
	t.Cleanup(func() { migrationsDir = "migrations" })
	migrated = false

	if err := WriteJobsToDatabase(t.Context(), []jobs.Job{{Title: "x", URL: "https://example.com/x"}}); err == nil {
		t.Fatal("WriteJobsToDatabase without migrations succeeded")
	}
}
