package server

import (
	"fmt"
	"net/http"
	"testing"

	"main/database"
	"main/internal/faultdb"
	"main/jobs"
	"main/llm"
)

// faultAPI is newAPI on a pool whose operations the returned injector can fail.
func faultAPI(t *testing.T) (*api, *faultdb.Injector) {
	t.Helper()
	a := newAPI(t)

	pool, inj, err := faultdb.Open(testDBConnString())
	if err != nil {
		t.Fatal(err)
	}

	prev := database.SetDB(pool)
	t.Cleanup(func() {
		database.SetDB(prev)
		pool.Close()
	})

	return a, inj
}

type faultRoute struct {
	name string
	// prepare runs unarmed and returns the request to send.
	prepare func() (method, path string, body any)
	// upload sends the resume as multipart instead of body.
	upload bool
	// lenient routes log some faults and still answer 2xx by design.
	lenient bool
}

func TestFaultsRoutes(t *testing.T) {
	a, inj := faultAPI(t)
	a.signup("fault-routes@example.com")
	pid := a.profile().ID

	reply := extractionJSON
	fakeLLM(t, func() string { return reply })

	giveActiveResume(t, pid, &llm.ExtractedResume{
		FullName:       "Ada",
		Skills:         []string{"Go"},
		WorkExperience: []llm.WorkExperienceEntry{{Company: "Acme", JobTitle: "Dev", Bullets: []string{"Built APIs"}}},
	})
	mustOK(t, database.SetActiveResume(t.Context(), pid, latestResume(t, pid)))
	ids := seedUniqueJobs(t, "Faultz", jobs.Job{Title: "Go Dev", Company: "Acme", Description: "Write Go"})
	jobID := ids["Go Dev"]
	mustOK(t, database.UpdateProfile(t.Context(), pid, &database.UpdateProfileRequest{EmailNotifications: true}))

	var n int
	unique := func() int { n++; return n }
	job := func(suffix string) string { return fmt.Sprintf("/api/jobs/%d%s", jobID, suffix) }
	tailor := func() {
		reply = `{"summary":"Go","skills":["Go"],"work_experience":[{"source":"W0","bullets":[{"text":"x","source":"W0.B0"}]}],"projects":[]}`
	}
	newQuiz := func() int64 {
		id, err := database.CreateTriviaQuiz(t.Context(), pid, "prompt", "Go", "easy", []llm.TriviaQuestion{{Question: "q", Options: []string{"a", "b"}, CorrectIndex: 1}})
		mustOK(t, err)
		return id
	}
	newWork := func() (int64, int64) {
		we, err := database.AddWorkExperience(t.Context(), pid, &database.AddWorkExperienceRequest{Company: fmt.Sprintf("Co%d", unique()), JobTitle: "Dev", JobType: "full_time"})
		mustOK(t, err)
		b, err := database.AddWorkExperienceBullet(t.Context(), pid, we, &database.AddWorkExperienceBulletRequest{Bullet: "b"})
		mustOK(t, err)
		return we, b
	}
	docx := testDocx(t, "Ada Lovelace, Go engineer")
	newResume := func() int64 {
		id, err := database.AddResume(t.Context(), pid, "r.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", docx)
		mustOK(t, err)
		return id
	}
	login := func() {
		a.must(http.StatusAccepted, "POST", "/api/profile/login", map[string]string{"email": "fault-routes@example.com", "password": "securepassword123"}, nil)
	}

	routes := []faultRoute{
		{name: "search jobs", lenient: true, prepare: func() (string, string, any) { return "GET", "/api/jobs?q=go", nil }},
		{name: "get job", lenient: true, prepare: func() (string, string, any) { return "GET", job(""), nil }},
		{name: "generate", prepare: func() (string, string, any) {
			reply = `{"cover_letter":"Dear","tailored_bullets":["x"]}`
			return "POST", job("/generate"), nil
		}},
		{name: "tailor", prepare: func() (string, string, any) { tailor(); return "POST", job("/tailored-resume"), nil }},
		{name: "get tailored", prepare: func() (string, string, any) { return "GET", job("/tailored-resume"), nil }},
		{name: "update tailored", prepare: func() (string, string, any) {
			return "PUT", job("/tailored-resume"), map[string]any{"full_name": "Ada", "skills": []string{"Go"}}
		}},
		{name: "apply", prepare: func() (string, string, any) { return "POST", job("/apply"), nil }},
		{name: "unapply", prepare: func() (string, string, any) { return "DELETE", job("/apply"), nil }},
		{name: "generate trivia", prepare: func() (string, string, any) {
			reply = triviaJSON(5)
			return "POST", "/api/trivia", map[string]any{"source": "prompt", "prompt": "Go channels", "count": 5, "difficulty": "easy"}
		}},
		{name: "list trivia", prepare: func() (string, string, any) { newQuiz(); return "GET", "/api/trivia", nil }},
		{name: "get trivia", prepare: func() (string, string, any) { return "GET", fmt.Sprintf("/api/trivia/%d", newQuiz()), nil }},
		{name: "answer trivia", prepare: func() (string, string, any) {
			return "PUT", fmt.Sprintf("/api/trivia/%d/answers", newQuiz()), map[string]any{"answers": []int{1}}
		}},
		{name: "retake trivia", prepare: func() (string, string, any) { return "POST", fmt.Sprintf("/api/trivia/%d/retake", newQuiz()), nil }},
		{name: "delete trivia", prepare: func() (string, string, any) { return "DELETE", fmt.Sprintf("/api/trivia/%d", newQuiz()), nil }},
		{name: "tags", prepare: func() (string, string, any) { return "GET", "/api/tags", nil }},
		{name: "unsubscribe", prepare: func() (string, string, any) {
			return "GET", fmt.Sprintf("/api/digest/unsubscribe?profile_id=%d&token=%s", pid, unsubscribeTokenFor(pid)), nil
		}},
		{name: "create profile", prepare: func() (string, string, any) {
			return "POST", "/api/profile/create", map[string]string{"email": fmt.Sprintf("fault%d@example.com", unique()), "password": "securepassword123"}
		}},
		{name: "login", prepare: func() (string, string, any) {
			return "POST", "/api/profile/login", map[string]string{"email": "fault-routes@example.com", "password": "securepassword123"}
		}},
		{name: "request reset", lenient: true, prepare: func() (string, string, any) {
			return "POST", "/api/profile/password-reset", map[string]string{"email": "fault-routes@example.com"}
		}},
		{name: "confirm reset", prepare: func() (string, string, any) {
			token, err := database.CreatePasswordResetToken(t.Context(), pid)
			mustOK(t, err)
			return "POST", "/api/profile/password-reset/confirm", map[string]string{"token": token, "password": "securepassword123"}
		}},
		{name: "refresh", lenient: true, prepare: func() (string, string, any) { login(); return "POST", "/api/profile/refresh", nil }},
		{name: "logout", lenient: true, prepare: func() (string, string, any) { login(); return "POST", "/api/profile/logout", nil }},
		{name: "get profile", prepare: func() (string, string, any) { return "GET", "/api/profile", nil }},
		{name: "update profile", prepare: func() (string, string, any) {
			return "PUT", "/api/profile", map[string]any{"name": "Ada", "email_notifications": true}
		}},
		{name: "add education", prepare: func() (string, string, any) {
			return "POST", "/api/profile/education", map[string]any{"school_name": fmt.Sprintf("S%d", unique()), "major": "CS", "degree": "BS"}
		}},
		{name: "update education", prepare: func() (string, string, any) {
			id, err := database.AddEducation(t.Context(), pid, &database.AddEducationRequest{SchoolName: fmt.Sprintf("S%d", unique()), Major: "CS", Degree: "BS"})
			mustOK(t, err)
			return "PUT", fmt.Sprintf("/api/profile/education/%d", id), map[string]any{"school_name": fmt.Sprintf("S%d", unique()), "major": "CS", "degree": "BS"}
		}},
		{name: "delete education", prepare: func() (string, string, any) {
			id, err := database.AddEducation(t.Context(), pid, &database.AddEducationRequest{SchoolName: fmt.Sprintf("S%d", unique()), Major: "CS", Degree: "BS"})
			mustOK(t, err)
			return "DELETE", fmt.Sprintf("/api/profile/education/%d", id), nil
		}},
		{name: "add skill", prepare: func() (string, string, any) {
			return "POST", "/api/profile/skills", map[string]any{"skill": fmt.Sprintf("s%d", unique())}
		}},
		{name: "delete skill", prepare: func() (string, string, any) {
			id, err := database.AddSkill(t.Context(), pid, &database.AddSkillRequest{Skill: fmt.Sprintf("s%d", unique())})
			mustOK(t, err)
			return "DELETE", fmt.Sprintf("/api/profile/skills/%d", id), nil
		}},
		{name: "add work", prepare: func() (string, string, any) {
			return "POST", "/api/profile/work-experience", map[string]any{"company": fmt.Sprintf("C%d", unique()), "job_title": "Dev", "job_type": "full_time"}
		}},
		{name: "update work", prepare: func() (string, string, any) {
			we, _ := newWork()
			return "PUT", fmt.Sprintf("/api/profile/work-experience/%d", we), map[string]any{"company": fmt.Sprintf("C%d", unique()), "job_title": "Dev", "job_type": "full_time"}
		}},
		{name: "delete work", prepare: func() (string, string, any) {
			we, _ := newWork()
			return "DELETE", fmt.Sprintf("/api/profile/work-experience/%d", we), nil
		}},
		{name: "add bullet", prepare: func() (string, string, any) {
			we, _ := newWork()
			return "POST", fmt.Sprintf("/api/profile/work-experience/%d/bullets", we), map[string]any{"bullet": "new"}
		}},
		{name: "update bullet", prepare: func() (string, string, any) {
			we, b := newWork()
			return "PUT", fmt.Sprintf("/api/profile/work-experience/%d/bullets/%d", we, b), map[string]any{"bullet": "edited"}
		}},
		{name: "delete bullet", prepare: func() (string, string, any) {
			we, b := newWork()
			return "DELETE", fmt.Sprintf("/api/profile/work-experience/%d/bullets/%d", we, b), nil
		}},
		{name: "upload resume", upload: true, lenient: true, prepare: func() (string, string, any) { return "POST", "/api/profile/resumes", nil }},
		{name: "replace resume", upload: true, lenient: true, prepare: func() (string, string, any) {
			return "PUT", fmt.Sprintf("/api/profile/resumes/%d", newResume()), nil
		}},
		{name: "download resume", prepare: func() (string, string, any) {
			return "GET", fmt.Sprintf("/api/profile/resumes/%d/download", newResume()), nil
		}},
		{name: "delete resume", prepare: func() (string, string, any) {
			return "DELETE", fmt.Sprintf("/api/profile/resumes/%d", newResume()), nil
		}},
		{name: "get extraction", prepare: func() (string, string, any) {
			id := newResume()
			mustOK(t, database.UpsertResumeExtractionPending(t.Context(), id))
			return "GET", fmt.Sprintf("/api/profile/resumes/%d/extraction", id), nil
		}},
		{name: "run extraction", lenient: true, prepare: func() (string, string, any) {
			reply = extractionJSON
			return "POST", fmt.Sprintf("/api/profile/resumes/%d/extraction", newResume()), nil
		}},
		{name: "run failed extraction", lenient: true, prepare: func() (string, string, any) {
			id, err := database.AddResume(t.Context(), pid, "bad.pdf", "application/pdf", []byte("not a pdf"))
			mustOK(t, err)
			return "POST", fmt.Sprintf("/api/profile/resumes/%d/extraction", id), nil
		}},
		{name: "activate resume", prepare: func() (string, string, any) {
			return "POST", fmt.Sprintf("/api/profile/resumes/%d/activate", newResume()), nil
		}},
	}

	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			var method, path string
			var body any
			var status int

			faultdb.SweepEach(t, inj, func() { method, path, body = route.prepare() }, func() {
				if route.upload {
					status, _ = a.upload(method, path, "cv.docx", docx, nil)
				} else {
					status, _ = a.do(method, path, body)
				}
			}, func(t *testing.T) {
				if status < 500 && !route.lenient {
					t.Errorf("%s %s = %d, want 5xx", method, path, status)
				}
			})
		})
	}
}

func mustOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func latestResume(t *testing.T, profileID int64) int64 {
	t.Helper()
	var id int64
	mustOK(t, database.DB().QueryRow("SELECT max(id) FROM profile_resumes WHERE profile_id = $1", profileID).Scan(&id))
	return id
}

func TestRefreshAfterProfileDeleted(t *testing.T) {
	a := newAPI(t).signup("vanish@example.com")
	id := a.profile().ID

	// Its refresh tokens cascade-delete with it.
	if _, err := database.DB().Exec("DELETE FROM profiles WHERE id = $1", id); err != nil {
		t.Fatal(err)
	}

	a.must(http.StatusUnauthorized, "POST", "/api/profile/refresh", nil, nil)
}
