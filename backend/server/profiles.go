package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"main/database"
	"main/utils"
	"net/http"
	"strconv"
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

func handleGetProfile(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromContext(r.Context())

	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	profile, err := database.GetProfile(r.Context(), profileID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "profile not found")
			return
		}

		log.Printf("get profile: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get profile")
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

func handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromContext(r.Context())

	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	req := &database.UpdateProfileRequest{}

	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if err := database.UpdateProfile(r.Context(), profileID, req); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "profile not found")
			return
		}

		log.Printf("update profile: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func handleAddEducation(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromContext(r.Context())

	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	req := &database.AddEducationRequest{}

	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	id, err := database.AddEducation(r.Context(), profileID, req)

	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func handleDeleteEducation(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromContext(r.Context())

	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)

	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid education id")
		return
	}

	if err := database.DeleteEducation(r.Context(), profileID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "education entry not found")
			return
		}

		log.Printf("delete education: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete education entry")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func handleAddSkill(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromContext(r.Context())

	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	req := &database.AddSkillRequest{}

	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	id, err := database.AddSkill(r.Context(), profileID, req)

	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func handleDeleteSkill(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromContext(r.Context())

	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)

	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid skill id")
		return
	}

	if err := database.DeleteSkill(r.Context(), profileID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "skill not found")
			return
		}

		log.Printf("delete skill: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete skill")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func handleAddWorkExperience(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromContext(r.Context())

	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	req := &database.AddWorkExperienceRequest{}

	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	id, err := database.AddWorkExperience(r.Context(), profileID, req)

	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func handleDeleteWorkExperience(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromContext(r.Context())

	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)

	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid work experience id")
		return
	}

	if err := database.DeleteWorkExperience(r.Context(), profileID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "work experience entry not found")
			return
		}

		log.Printf("delete work experience: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete work experience entry")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func handleAddWorkExperienceBullet(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromContext(r.Context())

	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	workExperienceID, err := strconv.ParseInt(r.PathValue("workExperienceId"), 10, 64)

	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid work experience id")
		return
	}

	req := &database.AddWorkExperienceBulletRequest{}

	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	id, err := database.AddWorkExperienceBullet(r.Context(), profileID, workExperienceID, req)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "work experience entry not found")
			return
		}

		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func handleDeleteWorkExperienceBullet(w http.ResponseWriter, r *http.Request) {
	profileID, ok := profileIDFromContext(r.Context())

	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	workExperienceID, err := strconv.ParseInt(r.PathValue("workExperienceId"), 10, 64)

	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid work experience id")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)

	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid bullet id")
		return
	}

	if err := database.DeleteWorkExperienceBullet(r.Context(), profileID, workExperienceID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "bullet not found")
			return
		}

		log.Printf("delete work experience bullet: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete bullet")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
