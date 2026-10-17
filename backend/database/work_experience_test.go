package database

import (
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
)

func newNamedProfile(t *testing.T, email string) int64 {
	t.Helper()

	id, err := CreateProfile(t.Context(), &ProfileRequest{Email: email, Password: "securepassword123"})
	if err != nil {
		t.Fatalf("create profile %s: %v", email, err)
	}

	return id
}

func TestParseJobType(t *testing.T) {
	tests := []struct {
		in     string
		want   JobType
		wantOK bool
	}{
		{"", JobTypeUnknown, true},
		{"unknown", JobTypeUnknown, true},
		{"contract", JobTypeContract, true},
		{"internship", JobTypeInternship, true},
		{"part_time", JobTypePartTime, true},
		{"full_time", JobTypeFullTime, true},
		{"Full_Time", JobTypeUnknown, false},
		{"freelance", JobTypeUnknown, false},
	}

	for _, tt := range tests {
		got, ok := ParseJobType(tt.in)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("ParseJobType(%q) = (%v, %v), want (%v, %v)", tt.in, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestJobTypeJSONRoundTrip(t *testing.T) {
	for _, jt := range []JobType{JobTypeUnknown, JobTypeContract, JobTypeInternship, JobTypePartTime, JobTypeFullTime} {
		raw, err := json.Marshal(jt)
		if err != nil {
			t.Fatalf("marshal %v: %v", jt, err)
		}

		if want := `"` + jt.String() + `"`; string(raw) != want {
			t.Errorf("Marshal(%v) = %s, want %s", jt, raw, want)
		}

		var back JobType
		if err := json.Unmarshal(raw, &back); err != nil {
			t.Fatalf("unmarshal %s: %v", raw, err)
		}

		if back != jt {
			t.Errorf("round trip %v -> %v", jt, back)
		}
	}

	var jt JobType = JobTypeFullTime
	if err := json.Unmarshal([]byte(`"bogus"`), &jt); err != nil || jt != JobTypeUnknown {
		t.Errorf("Unmarshal(bogus) = (%v, %v), want (unknown, nil)", jt, err)
	}

	if err := json.Unmarshal([]byte(`3`), &jt); err == nil {
		t.Error("Unmarshal(number) = nil error, want error")
	}

	if got := JobType(99).String(); got != "unknown" {
		t.Errorf("JobType(99).String() = %q", got)
	}
}

func TestWorkExperienceCRUD(t *testing.T) {
	newTestDB(t)
	ctx := t.Context()
	id := newTestProfile(t)

	older, err := AddWorkExperience(ctx, id, &AddWorkExperienceRequest{
		Company: "Old Co", JobTitle: "Intern", JobType: "internship", StartDate: "2019-06-01", EndDate: "2019-08-31",
	})
	if err != nil {
		t.Fatalf("AddWorkExperience(older): %v", err)
	}

	newer, err := AddWorkExperience(ctx, id, &AddWorkExperienceRequest{
		Company: "New Co", JobTitle: "Engineer", JobType: "full_time", Location: "Remote", StartDate: "2022-01-01",
	})
	if err != nil {
		t.Fatalf("AddWorkExperience(newer): %v", err)
	}

	profile, err := GetProfile(ctx, id)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}

	if len(profile.WorkExperience) != 2 {
		t.Fatalf("WorkExperience = %d entries, want 2", len(profile.WorkExperience))
	}

	first := profile.WorkExperience[0]
	if first.ID != newer || first.JobType != JobTypeFullTime || first.Location == nil || *first.Location != "Remote" ||
		first.StartDate == nil || *first.StartDate != "2022-01-01" || first.EndDate != nil {
		t.Errorf("newest entry = %+v, want New Co full_time Remote from 2022-01-01", first)
	}

	second := profile.WorkExperience[1]
	if second.ID != older || second.Location != nil || second.EndDate == nil || *second.EndDate != "2019-08-31" {
		t.Errorf("older entry = %+v", second)
	}

	if err := UpdateWorkExperience(ctx, id, older, &AddWorkExperienceRequest{
		Company: "Old Co", JobTitle: "Senior Intern", JobType: "contract", Location: "NYC",
	}); err != nil {
		t.Fatalf("UpdateWorkExperience: %v", err)
	}

	profile, _ = GetProfile(ctx, id)
	var updated ProfileWorkExperience
	for _, we := range profile.WorkExperience {
		if we.ID == older {
			updated = we
		}
	}

	if updated.JobTitle != "Senior Intern" || updated.JobType != JobTypeContract || updated.StartDate != nil {
		t.Errorf("updated = %+v", updated)
	}

	if err := DeleteWorkExperience(ctx, id, older); err != nil {
		t.Fatalf("DeleteWorkExperience: %v", err)
	}

	if err := DeleteWorkExperience(ctx, id, older); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("second delete err = %v, want sql.ErrNoRows", err)
	}

	profile, _ = GetProfile(ctx, id)
	if len(profile.WorkExperience) != 1 || profile.WorkExperience[0].ID != newer {
		t.Errorf("after delete = %+v", profile.WorkExperience)
	}
}

func TestAddWorkExperienceUpsertsOnSameStart(t *testing.T) {
	newTestDB(t)
	ctx := t.Context()
	id := newTestProfile(t)

	req := &AddWorkExperienceRequest{Company: "Acme", JobTitle: "Dev", JobType: "contract", StartDate: "2020-01-01"}

	a, err := AddWorkExperience(ctx, id, req)
	if err != nil {
		t.Fatal(err)
	}

	req.JobType = "full_time"
	b, err := AddWorkExperience(ctx, id, req)
	if err != nil {
		t.Fatal(err)
	}

	if a != b {
		t.Errorf("upsert returned new id %d, want %d", b, a)
	}

	profile, _ := GetProfile(ctx, id)
	if len(profile.WorkExperience) != 1 || profile.WorkExperience[0].JobType != JobTypeFullTime {
		t.Errorf("WorkExperience = %+v, want one full_time entry", profile.WorkExperience)
	}
}

func TestWorkExperienceValidation(t *testing.T) {
	newTestDB(t)
	ctx := t.Context()
	id := newTestProfile(t)

	bad := []AddWorkExperienceRequest{
		{JobTitle: "Dev"},
		{Company: "Acme"},
		{Company: "Acme", JobTitle: "Dev", JobType: "freelance"},
		{Company: "Acme", JobTitle: "Dev", StartDate: "01/02/2020"},
		{Company: "Acme", JobTitle: "Dev", EndDate: "nope"},
	}

	for _, req := range bad {
		if _, err := AddWorkExperience(ctx, id, &req); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("AddWorkExperience(%+v) err = %v, want ErrInvalidInput", req, err)
		}

		if err := UpdateWorkExperience(ctx, id, 1, &req); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("UpdateWorkExperience(%+v) err = %v, want ErrInvalidInput", req, err)
		}
	}

	if err := UpdateWorkExperience(ctx, id, 999, &AddWorkExperienceRequest{Company: "A", JobTitle: "B"}); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("UpdateWorkExperience(missing) err = %v, want sql.ErrNoRows", err)
	}
}

func TestWorkExperienceBullets(t *testing.T) {
	newTestDB(t)
	ctx := t.Context()
	id := newTestProfile(t)
	other := newNamedProfile(t, "other@example.com")

	weID, err := AddWorkExperience(ctx, id, &AddWorkExperienceRequest{Company: "Acme", JobTitle: "Dev"})
	if err != nil {
		t.Fatal(err)
	}

	second, err := AddWorkExperienceBullet(ctx, id, weID, &AddWorkExperienceBulletRequest{Bullet: "Second", Position: 1})
	if err != nil {
		t.Fatalf("AddWorkExperienceBullet: %v", err)
	}

	first, err := AddWorkExperienceBullet(ctx, id, weID, &AddWorkExperienceBulletRequest{Bullet: "First", Position: 0})
	if err != nil {
		t.Fatalf("AddWorkExperienceBullet: %v", err)
	}

	bullets, err := listWorkExperienceBullets(ctx, weID)
	if err != nil {
		t.Fatal(err)
	}

	if len(bullets) != 2 || bullets[0].ID != first || bullets[1].ID != second {
		t.Fatalf("bullets = %+v, want ordered by position", bullets)
	}

	if _, err := AddWorkExperienceBullet(ctx, id, weID, &AddWorkExperienceBulletRequest{}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("empty bullet err = %v, want ErrInvalidInput", err)
	}

	if _, err := AddWorkExperienceBullet(ctx, other, weID, &AddWorkExperienceBulletRequest{Bullet: "x"}); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("other profile add err = %v, want sql.ErrNoRows", err)
	}

	if err := UpdateWorkExperienceBullet(ctx, id, weID, first, &UpdateWorkExperienceBulletRequest{Bullet: "Edited"}); err != nil {
		t.Fatalf("UpdateWorkExperienceBullet: %v", err)
	}

	if err := UpdateWorkExperienceBullet(ctx, id, weID, first, &UpdateWorkExperienceBulletRequest{}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("empty update err = %v, want ErrInvalidInput", err)
	}

	if err := UpdateWorkExperienceBullet(ctx, other, weID, first, &UpdateWorkExperienceBulletRequest{Bullet: "Hijack"}); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("other profile update err = %v, want sql.ErrNoRows", err)
	}

	bullets, _ = listWorkExperienceBullets(ctx, weID)
	if bullets[0].Bullet != "Edited" {
		t.Errorf("bullet text = %q, want Edited", bullets[0].Bullet)
	}

	if err := DeleteWorkExperienceBullet(ctx, other, weID, first); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("other profile delete err = %v, want sql.ErrNoRows", err)
	}

	if err := DeleteWorkExperienceBullet(ctx, id, weID, first); err != nil {
		t.Fatalf("DeleteWorkExperienceBullet: %v", err)
	}

	if err := DeleteWorkExperienceBullet(ctx, id, weID, first); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("second delete err = %v, want sql.ErrNoRows", err)
	}

	// Deleting the entry takes its remaining bullets with it.
	if err := DeleteWorkExperience(ctx, id, weID); err != nil {
		t.Fatal(err)
	}

	bullets, _ = listWorkExperienceBullets(ctx, weID)
	if len(bullets) != 0 {
		t.Errorf("bullets after entry delete = %+v", bullets)
	}
}

func TestDeleteWorkExperienceWrongProfile(t *testing.T) {
	newTestDB(t)
	ctx := t.Context()
	id := newTestProfile(t)
	other := newNamedProfile(t, "other@example.com")

	weID, err := AddWorkExperience(ctx, id, &AddWorkExperienceRequest{Company: "Acme", JobTitle: "Dev"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := AddWorkExperienceBullet(ctx, id, weID, &AddWorkExperienceBulletRequest{Bullet: "Mine"}); err != nil {
		t.Fatal(err)
	}

	if err := DeleteWorkExperience(ctx, other, weID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("err = %v, want sql.ErrNoRows", err)
	}

	// The owner's bullets survive another profile's delete attempt.
	if bullets, _ := listWorkExperienceBullets(ctx, weID); len(bullets) != 1 {
		t.Errorf("bullets after foreign delete = %+v, want 1", bullets)
	}

	if err := UpdateWorkExperience(ctx, other, weID, &AddWorkExperienceRequest{Company: "A", JobTitle: "B"}); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("update err = %v, want sql.ErrNoRows", err)
	}
}
