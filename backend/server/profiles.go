package server

import (
	"encoding/json"
	"main/database"
	"main/utils"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func createToken(profileID int64) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256,
		jwt.MapClaims{
			"profileID": profileID,
			"exp":       time.Now().Add(time.Hour * 24).Unix(),
		})

	tokenString, err := token.SignedString([]byte(utils.GetEnv()["SECRET_KEY"]))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func handleLoginProfile(w http.ResponseWriter, r *http.Request) {
	req := &database.ProfileRequest{}
	err := json.NewDecoder(r.Body).Decode(&req)

	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "Could not login to profile!"})
		return
	}

	id, hash, err := database.GetProfileByEmail(req.Email)

	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "Could not login to profile!"})
		return
	}

	if !database.CheckPasswordHash(req.Password, hash) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "Incorrect password"})
		return
	}

	token, err := createToken(id)

	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "Could not create token!"})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"token": token})
}

func handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	req := &database.ProfileRequest{}
	err := json.NewDecoder(r.Body).Decode(&req)

	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "Invalid request!"})
		return
	}

	profileID, err := database.CreateProfile(req)

	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "Could not create profile!"})
		return
	}

	token, err := createToken(profileID)

	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "Could not create token!"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"token": token})
}
