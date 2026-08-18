package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func newTestProfile(t *testing.T) int64 {
	t.Helper()

	id, err := CreateProfile(&ProfileRequest{Email: "test@example.com", Password: "securepassword123"})

	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	return id
}

func TestGetProfile(t *testing.T) {
	newTestDB(t)
	id := newTestProfile(t)

	profile, err := GetProfile(context.Background(), id)

	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}

	if profile.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", profile.Email, "test@example.com")
	}

	if profile.Name != "" || profile.Address != "" {
		t.Errorf("expected empty optional fields, got Name=%q Address=%q", profile.Name, profile.Address)
	}

	if len(profile.Education) != 0 || len(profile.Skills) != 0 {
		t.Errorf("expected no education/skills, got %d/%d", len(profile.Education), len(profile.Skills))
	}
}

func TestGetProfileNotFound(t *testing.T) {
	newTestDB(t)

	if err := CreateTables(); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	_, err := GetProfile(context.Background(), 999)

	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("err = %v, want sql.ErrNoRows", err)
	}
}

func TestUpdateProfile(t *testing.T) {
	newTestDB(t)
	id := newTestProfile(t)

	req := &UpdateProfileRequest{
		Name:      "Ada Lovelace",
		Address:   "London",
		LinkedIn:  "linkedin.com/in/ada",
		GitHub:    "github.com/ada",
		Portfolio: "ada.dev",
	}

	if err := UpdateProfile(context.Background(), id, req); err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}

	profile, err := GetProfile(context.Background(), id)

	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}

	if profile.Name != req.Name || profile.Address != req.Address || profile.LinkedIn != req.LinkedIn ||
		profile.GitHub != req.GitHub || profile.Portfolio != req.Portfolio {
		t.Errorf("profile = %+v, want fields from %+v", profile, req)
	}
}

func TestUpdateProfileNotFound(t *testing.T) {
	newTestDB(t)

	if err := CreateTables(); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	err := UpdateProfile(context.Background(), 999, &UpdateProfileRequest{Name: "Nobody"})

	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("err = %v, want sql.ErrNoRows", err)
	}
}

func TestAddAndDeleteEducation(t *testing.T) {
	newTestDB(t)
	id := newTestProfile(t)

	gpa := 3.9
	req := &AddEducationRequest{
		SchoolName: "MIT",
		Major:      "Computer Science",
		Degree:     "BS",
		GPA:        &gpa,
		StartDate:  "2018-09-01",
		EndDate:    "2022-05-15",
	}

	educationID, err := AddEducation(context.Background(), id, req)

	if err != nil {
		t.Fatalf("AddEducation: %v", err)
	}

	profile, err := GetProfile(context.Background(), id)

	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}

	if len(profile.Education) != 1 {
		t.Fatalf("Education = %v, want 1 entry", profile.Education)
	}

	got := profile.Education[0]

	if got.ID != educationID || got.SchoolName != req.SchoolName || got.Major != req.Major || got.Degree != req.Degree {
		t.Errorf("education = %+v, want fields from %+v", got, req)
	}

	if got.GPA == nil || *got.GPA != gpa {
		t.Errorf("GPA = %v, want %v", got.GPA, gpa)
	}

	if got.StartDate == nil || *got.StartDate != req.StartDate {
		t.Errorf("StartDate = %v, want %v", got.StartDate, req.StartDate)
	}

	if got.EndDate == nil || *got.EndDate != req.EndDate {
		t.Errorf("EndDate = %v, want %v", got.EndDate, req.EndDate)
	}

	if err := DeleteEducation(context.Background(), id, educationID); err != nil {
		t.Fatalf("DeleteEducation: %v", err)
	}

	profile, err = GetProfile(context.Background(), id)

	if err != nil {
		t.Fatalf("GetProfile after delete: %v", err)
	}

	if len(profile.Education) != 0 {
		t.Errorf("Education after delete = %v, want none", profile.Education)
	}
}

func TestAddEducationRequiresFields(t *testing.T) {
	newTestDB(t)
	id := newTestProfile(t)

	_, err := AddEducation(context.Background(), id, &AddEducationRequest{SchoolName: "MIT"})

	if err == nil {
		t.Fatal("expected error for missing major/degree")
	}
}

func TestDeleteEducationWrongProfile(t *testing.T) {
	newTestDB(t)
	id := newTestProfile(t)

	educationID, err := AddEducation(context.Background(), id, &AddEducationRequest{
		SchoolName: "MIT", Major: "CS", Degree: "BS",
	})

	if err != nil {
		t.Fatalf("AddEducation: %v", err)
	}

	err = DeleteEducation(context.Background(), id+1, educationID)

	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("err = %v, want sql.ErrNoRows", err)
	}
}

func TestAddAndDeleteSkill(t *testing.T) {
	newTestDB(t)
	id := newTestProfile(t)

	skillID, err := AddSkill(context.Background(), id, &AddSkillRequest{Skill: "Go"})

	if err != nil {
		t.Fatalf("AddSkill: %v", err)
	}

	profile, err := GetProfile(context.Background(), id)

	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}

	if len(profile.Skills) != 1 || profile.Skills[0].ID != skillID || profile.Skills[0].Skill != "Go" {
		t.Errorf("Skills = %v, want [{%d Go}]", profile.Skills, skillID)
	}

	if err := DeleteSkill(context.Background(), id, skillID); err != nil {
		t.Fatalf("DeleteSkill: %v", err)
	}

	profile, err = GetProfile(context.Background(), id)

	if err != nil {
		t.Fatalf("GetProfile after delete: %v", err)
	}

	if len(profile.Skills) != 0 {
		t.Errorf("Skills after delete = %v, want none", profile.Skills)
	}
}

func TestAddSkillRequiresValue(t *testing.T) {
	newTestDB(t)
	id := newTestProfile(t)

	_, err := AddSkill(context.Background(), id, &AddSkillRequest{Skill: ""})

	if err == nil {
		t.Fatal("expected error for empty skill")
	}
}
