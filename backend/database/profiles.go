package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	emailverifier "github.com/AfterShip/email-verifier"
	"golang.org/x/crypto/bcrypt"
)

const dateLayout = "2006-01-02"

type Profile struct {
	ID                 int64                   `json:"id"`
	Email              string                  `json:"email"`
	Name               string                  `json:"name"`
	Address            string                  `json:"address"`
	LinkedIn           string                  `json:"linked_in"`
	GitHub             string                  `json:"github"`
	Portfolio          string                  `json:"portfolio"`
	EmailNotifications bool                    `json:"email_notifications"`
	Education          []ProfileEducation      `json:"education"`
	Skills             []ProfileSkill          `json:"skills"`
	WorkExperience     []ProfileWorkExperience `json:"work_experience"`
	Resumes            []ProfileResume         `json:"resumes"`
}

type UpdateProfileRequest struct {
	Name               string `json:"name"`
	Address            string `json:"address"`
	LinkedIn           string `json:"linked_in"`
	GitHub             string `json:"github"`
	Portfolio          string `json:"portfolio"`
	EmailNotifications bool   `json:"email_notifications"`
}

var verifier = emailverifier.NewVerifier()

type ProfileRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))

	return err == nil
}

// Compared against on a lookup miss so login timing doesn't leak whether an email exists.
var DummyPasswordHash = mustHashPassword("this is not anybody's real password")

func mustHashPassword(password string) string {
	hash, err := HashPassword(password)

	if err != nil {
		panic(fmt.Errorf("hash dummy password: %w", err))
	}

	return hash
}

var ErrInvalidProfile = errors.New("invalid profile request")
var errEmailVerificationTimeout = errors.New("email verification timed out")

const emailVerificationTimeout = 8 * time.Second

func verifyEmail(email string) (*emailverifier.Result, error) {
	type outcome struct {
		result *emailverifier.Result
		err    error
	}

	done := make(chan outcome, 1)

	go func() {
		result, err := verifier.Verify(email)
		done <- outcome{result, err}
	}()

	select {
	case o := <-done:
		return o.result, o.err
	case <-time.After(emailVerificationTimeout):
		return nil, errEmailVerificationTimeout
	}
}

func GetProfileByEmail(email string) (int64, string, error) {
	db, err := GetDb()

	if err != nil {
		return 0, "", err
	}

	var id int64
	var password string

	if err := db.QueryRow("SELECT id, password FROM profiles WHERE email=$1", email).Scan(&id, &password); err != nil {
		return 0, "", err
	}

	return id, password, nil
}

func CreateProfile(req *ProfileRequest) (int64, error) {
	if len(req.Email) == 0 || len(req.Password) == 0 {
		return 0, fmt.Errorf("%w: email and password are required", ErrInvalidProfile)
	}
	if len(req.Password) < 8 {
		return 0, fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidProfile)
	}

	result, err := verifyEmail(req.Email)
	if errors.Is(err, errEmailVerificationTimeout) {
		if !verifier.ParseAddress(req.Email).Valid {
			return 0, fmt.Errorf("%w: email is invalid or undeliverable", ErrInvalidProfile)
		}
	} else if err != nil {
		return 0, fmt.Errorf("verify email: %w", err)
	} else if !result.Syntax.Valid || !result.HasMxRecords || result.Disposable {
		return 0, fmt.Errorf("%w: email is invalid or undeliverable", ErrInvalidProfile)
	}

	db, err := GetDb()

	if err != nil {
		return 0, err
	}

	if err := CreateTables(); err != nil {
		return 0, err
	}

	tx, err := db.Begin()

	if err != nil {
		return 0, err
	}

	defer tx.Rollback()

	hashedPassword, err := HashPassword(req.Password)

	if err != nil {
		return 0, err
	}

	var profileID int64

	if err := tx.QueryRow(insertStatements["profiles"], req.Email, hashedPassword).Scan(&profileID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("%w: email already registered", ErrInvalidProfile)
		}

		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return profileID, nil
}

func ProfileExists(ctx context.Context, id int64) (bool, error) {
	db, err := GetDb()
	if err != nil {
		return false, err
	}

	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM profiles WHERE id = $1)`, id).Scan(&exists); err != nil {
		return false, err
	}

	return exists, nil
}

func GetProfile(ctx context.Context, id int64) (*Profile, error) {
	db, err := GetDb()

	if err != nil {
		return nil, err
	}

	profile := &Profile{ID: id}

	row := db.QueryRowContext(ctx, `SELECT email, COALESCE(name, ''), COALESCE(address, ''),
		COALESCE(linked_in, ''), COALESCE(github, ''), COALESCE(portfolio, ''), email_notifications_enabled
		FROM profiles WHERE id = $1`, id)

	if err := row.Scan(&profile.Email, &profile.Name, &profile.Address, &profile.LinkedIn, &profile.GitHub,
		&profile.Portfolio, &profile.EmailNotifications); err != nil {
		return nil, err
	}

	education, err := listEducation(ctx, db, id)

	if err != nil {
		return nil, fmt.Errorf("list education: %w", err)
	}

	profile.Education = education

	skills, err := listSkills(ctx, db, id)

	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}

	profile.Skills = skills

	workExperience, err := listWorkExperience(ctx, db, id)

	if err != nil {
		return nil, fmt.Errorf("list work experience: %w", err)
	}

	profile.WorkExperience = workExperience

	resumes, err := ListResumes(ctx, id)

	if err != nil {
		return nil, fmt.Errorf("list resumes: %w", err)
	}

	profile.Resumes = resumes

	return profile, nil
}

func UpdateProfile(ctx context.Context, id int64, req *UpdateProfileRequest) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	result, err := db.ExecContext(ctx, `UPDATE profiles SET name = $1, address = $2, linked_in = $3, github = $4,
		portfolio = $5, email_notifications_enabled = $6 WHERE id = $7`,
		req.Name, req.Address, req.LinkedIn, req.GitHub, req.Portfolio, req.EmailNotifications, id)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()

	if err != nil {
		return err
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func parseOptionalDate(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}

	parsed, err := time.Parse(dateLayout, raw)

	if err != nil {
		return nil, err
	}

	return &parsed, nil
}
