package server

import (
	"fmt"
	"net/http"
	"testing"
)

type idResponse struct {
	ID int64 `json:"id"`
}

func TestAPIUpdateProfile(t *testing.T) {
	a := newAPI(t).signup("api-update@example.com")

	a.must(http.StatusOK, "PUT", "/api/profile", map[string]any{
		"name": "Ada", "address": "London", "linked_in": "https://linkedin.com/in/ada",
		"github": "https://github.com/ada", "portfolio": "https://ada.dev", "email_notifications": true,
	}, nil)

	p := a.profile()
	if p.Name != "Ada" || p.Address != "London" || p.GitHub != "https://github.com/ada" || !p.EmailNotifications {
		t.Errorf("profile = %+v", p)
	}

	a.must(http.StatusBadRequest, "PUT", "/api/profile", "{", nil)
}

func TestAPIEducation(t *testing.T) {
	a := newAPI(t).signup("api-edu@example.com")
	other := a.as().signup("api-edu-other@example.com")

	var created idResponse
	a.must(http.StatusCreated, "POST", "/api/profile/education",
		map[string]any{"school_name": "MIT", "major": "CS", "degree": "BS", "gpa": 3.8, "start_date": "2018-09-01"}, &created)

	path := fmt.Sprintf("/api/profile/education/%d", created.ID)

	a.must(http.StatusOK, "PUT", path, map[string]any{"school_name": "MIT", "major": "Math", "degree": "BS"}, nil)

	if edu := a.profile().Education; len(edu) != 1 || edu[0].Major != "Math" {
		t.Errorf("education = %+v", edu)
	}

	a.must(http.StatusBadRequest, "POST", "/api/profile/education", map[string]any{"school_name": "MIT"}, nil)
	a.must(http.StatusBadRequest, "POST", "/api/profile/education", "nope", nil)
	a.must(http.StatusBadRequest, "PUT", path, map[string]any{"school_name": "MIT", "major": "x", "degree": "y", "start_date": "bad"}, nil)
	a.must(http.StatusBadRequest, "PUT", "/api/profile/education/abc", map[string]any{}, nil)
	a.must(http.StatusBadRequest, "PUT", path, "{", nil)
	a.must(http.StatusBadRequest, "DELETE", "/api/profile/education/abc", nil, nil)

	other.must(http.StatusNotFound, "PUT", path, map[string]any{"school_name": "x", "major": "y", "degree": "z"}, nil)
	other.must(http.StatusNotFound, "DELETE", path, nil, nil)

	a.must(http.StatusOK, "DELETE", path, nil, nil)
	a.must(http.StatusNotFound, "DELETE", path, nil, nil)
}

func TestAPISkills(t *testing.T) {
	a := newAPI(t).signup("api-skills@example.com")
	other := a.as().signup("api-skills-other@example.com")

	var created idResponse
	a.must(http.StatusCreated, "POST", "/api/profile/skills", map[string]string{"skill": "Go"}, &created)

	if skills := a.profile().Skills; len(skills) != 1 || skills[0].Skill != "Go" {
		t.Errorf("skills = %+v", skills)
	}

	a.must(http.StatusBadRequest, "POST", "/api/profile/skills", map[string]string{"skill": ""}, nil)
	a.must(http.StatusBadRequest, "POST", "/api/profile/skills", "{", nil)
	a.must(http.StatusBadRequest, "DELETE", "/api/profile/skills/x", nil, nil)

	path := fmt.Sprintf("/api/profile/skills/%d", created.ID)
	other.must(http.StatusNotFound, "DELETE", path, nil, nil)
	a.must(http.StatusOK, "DELETE", path, nil, nil)
	a.must(http.StatusNotFound, "DELETE", path, nil, nil)
}

func TestAPIWorkExperienceAndBullets(t *testing.T) {
	a := newAPI(t).signup("api-work@example.com")
	other := a.as().signup("api-work-other@example.com")

	var we idResponse
	a.must(http.StatusCreated, "POST", "/api/profile/work-experience",
		map[string]any{"company": "Acme", "job_title": "Dev", "job_type": "full_time", "start_date": "2021-01-01"}, &we)

	wePath := fmt.Sprintf("/api/profile/work-experience/%d", we.ID)
	bulletsPath := wePath + "/bullets"

	var bullet idResponse
	a.must(http.StatusCreated, "POST", bulletsPath, map[string]any{"bullet": "Shipped it", "position": 0}, &bullet)
	bulletPath := fmt.Sprintf("%s/%d", bulletsPath, bullet.ID)

	a.must(http.StatusOK, "PUT", bulletPath, map[string]string{"bullet": "Shipped it twice"}, nil)
	a.must(http.StatusOK, "PUT", wePath, map[string]any{"company": "Acme", "job_title": "Senior Dev", "job_type": "contract"}, nil)

	entries := a.profile().WorkExperience
	if len(entries) != 1 || entries[0].JobTitle != "Senior Dev" || len(entries[0].Bullets) != 1 || entries[0].Bullets[0].Bullet != "Shipped it twice" {
		t.Fatalf("work experience = %+v", entries)
	}

	// Validation and malformed input.
	a.must(http.StatusBadRequest, "POST", "/api/profile/work-experience", map[string]any{"company": "Acme"}, nil)
	a.must(http.StatusBadRequest, "POST", "/api/profile/work-experience", map[string]any{"company": "A", "job_title": "B", "job_type": "gig"}, nil)
	a.must(http.StatusBadRequest, "POST", "/api/profile/work-experience", "{", nil)
	a.must(http.StatusBadRequest, "PUT", wePath, "{", nil)
	a.must(http.StatusBadRequest, "PUT", "/api/profile/work-experience/x", map[string]any{}, nil)
	a.must(http.StatusBadRequest, "DELETE", "/api/profile/work-experience/x", nil, nil)
	a.must(http.StatusBadRequest, "POST", bulletsPath, map[string]any{"bullet": ""}, nil)
	a.must(http.StatusBadRequest, "POST", bulletsPath, "{", nil)
	a.must(http.StatusBadRequest, "POST", "/api/profile/work-experience/x/bullets", map[string]any{"bullet": "b"}, nil)
	a.must(http.StatusBadRequest, "PUT", bulletPath, map[string]string{"bullet": ""}, nil)
	a.must(http.StatusBadRequest, "PUT", bulletPath, "{", nil)
	a.must(http.StatusBadRequest, "PUT", bulletsPath+"/x", map[string]string{"bullet": "b"}, nil)
	a.must(http.StatusBadRequest, "PUT", "/api/profile/work-experience/x/bullets/1", map[string]string{"bullet": "b"}, nil)
	a.must(http.StatusBadRequest, "DELETE", bulletsPath+"/x", nil, nil)
	a.must(http.StatusBadRequest, "DELETE", "/api/profile/work-experience/x/bullets/1", nil, nil)

	// Another profile can't touch any of it.
	other.must(http.StatusNotFound, "POST", bulletsPath, map[string]any{"bullet": "b"}, nil)
	other.must(http.StatusNotFound, "PUT", bulletPath, map[string]string{"bullet": "b"}, nil)
	other.must(http.StatusNotFound, "DELETE", bulletPath, nil, nil)
	other.must(http.StatusNotFound, "PUT", wePath, map[string]any{"company": "A", "job_title": "B"}, nil)
	other.must(http.StatusNotFound, "DELETE", wePath, nil, nil)

	a.must(http.StatusOK, "DELETE", bulletPath, nil, nil)
	a.must(http.StatusNotFound, "DELETE", bulletPath, nil, nil)
	a.must(http.StatusOK, "DELETE", wePath, nil, nil)
	a.must(http.StatusNotFound, "DELETE", wePath, nil, nil)
}
