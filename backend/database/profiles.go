package database

import (
	"fmt"

	emailverifier "github.com/AfterShip/email-verifier"
	"golang.org/x/crypto/bcrypt"
)

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
	// Basic validation of email and password first
	if len(req.Email) == 0 || len(req.Password) == 0 {
		return 0, fmt.Errorf("Email and password cannot be empty!")
	}

	result, err := verifier.Verify(req.Email)
	if err != nil {
		return 0, err
	}
	if !result.Syntax.Valid || !result.HasMxRecords || result.Disposable {
		return 0, fmt.Errorf("Email is invalid or undeliverable")
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

	insertStatements, err := prepareInsertStatements(tx)

	if err != nil {
		return 0, err
	}

	hashedPassword, err := HashPassword(req.Password)

	if err != nil {
		return 0, err
	}

	var profileID int64

	if err := insertStatements["profiles"].QueryRow(req.Email, hashedPassword).Scan(&profileID); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return profileID, nil
}
