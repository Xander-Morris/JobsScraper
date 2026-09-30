package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"main/database"
	"main/llm"
)

const (
	triviaSourceResume = "resume"
	triviaSourcePrompt = "prompt"
	triviaResumeTopic  = "Your resume"
	minTriviaPrompt    = 3
	maxTriviaPrompt    = 300
)

var (
	triviaCounts       = []int{5, 10, 15}
	triviaDifficulties = []string{"easy", "medium", "hard"}
)

type triviaRequest struct {
	Source     string `json:"source"`
	Prompt     string `json:"prompt"`
	Count      int    `json:"count"`
	Difficulty string `json:"difficulty"`
}

type triviaAnswersRequest struct {
	Answers []int `json:"answers"`
}

// parseTriviaRequest validates req in place, returning a caller-facing error.
func parseTriviaRequest(req *triviaRequest) error {
	req.Prompt = strings.TrimSpace(req.Prompt)

	switch req.Source {
	case triviaSourceResume:
		req.Prompt = ""
	case triviaSourcePrompt:
		length := utf8.RuneCountInString(req.Prompt)
		if length < minTriviaPrompt || length > maxTriviaPrompt {
			return errors.New("topic must be 3-300 characters")
		}
	default:
		return errors.New("source must be resume or prompt")
	}

	if !slices.Contains(triviaCounts, req.Count) {
		return errors.New("count must be 5, 10, or 15")
	}

	if !slices.Contains(triviaDifficulties, req.Difficulty) {
		return errors.New("difficulty must be easy, medium, or hard")
	}

	return nil
}

func writeTriviaQuiz(w http.ResponseWriter, r *http.Request, profileID, id int64, status int) {
	quiz, err := database.GetTriviaQuiz(r.Context(), profileID, id)
	if err != nil {
		writeDBError(w, "get trivia quiz", err, "quiz not found", "failed to load quiz")
		return
	}

	writeJSON(w, status, quiz)
}

func handleGenerateTrivia(w http.ResponseWriter, r *http.Request, profileID int64) {
	var req triviaRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid trivia request")
		return
	}

	if err := parseTriviaRequest(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	llmReq := llm.TriviaRequest{Prompt: req.Prompt, Count: req.Count, Difficulty: req.Difficulty}
	topic := req.Prompt

	if req.Source == triviaSourceResume {
		extraction, found, err := database.GetActiveResumeExtraction(r.Context(), profileID)
		if err != nil {
			slog.Error("generate trivia: get active resume extraction", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to load active resume")
			return
		}
		if !found {
			writeError(w, http.StatusNotFound, "no completed resume extraction found for your active resume")
			return
		}

		llmReq.Resume = &llm.ResumeProfile{
			Summary:        extraction.Summary,
			Skills:         extraction.Skills,
			WorkExperience: extraction.WorkExperience,
			Projects:       extraction.Projects,
		}
		topic = triviaResumeTopic
	}

	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(generationTimeout + 15*time.Second)); err != nil {
		slog.Error("generate trivia: extend write deadline", "error", err)
	}

	ctx, cancel := context.WithTimeout(r.Context(), generationTimeout)
	defer cancel()

	questions, err := llm.GenerateTrivia(ctx, llmReq)
	if err != nil {
		slog.Error("generate trivia", "profile_id", profileID, "source", req.Source, "error", err)
		writeError(w, http.StatusBadGateway, "failed to generate quiz")
		return
	}

	id, err := database.CreateTriviaQuiz(r.Context(), profileID, req.Source, topic, req.Difficulty, questions)
	if err != nil {
		slog.Error("generate trivia: save", "profile_id", profileID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save quiz")
		return
	}

	writeTriviaQuiz(w, r, profileID, id, http.StatusCreated)
}

func handleListTrivia(w http.ResponseWriter, r *http.Request, profileID int64) {
	quizzes, err := database.ListTriviaQuizzes(r.Context(), profileID)
	if err != nil {
		slog.Error("list trivia quizzes", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load quizzes")
		return
	}

	writeJSON(w, http.StatusOK, quizzes)
}

func handleGetTrivia(w http.ResponseWriter, r *http.Request, profileID int64) {
	id, ok := pathID(w, r, "id", "quiz")
	if !ok {
		return
	}

	writeTriviaQuiz(w, r, profileID, id, http.StatusOK)
}

func handleSaveTriviaAnswers(w http.ResponseWriter, r *http.Request, profileID int64) {
	id, ok := pathID(w, r, "id", "quiz")
	if !ok {
		return
	}

	var req triviaAnswersRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid answers")
		return
	}

	if err := database.SaveTriviaAnswers(r.Context(), profileID, id, req.Answers); err != nil {
		writeDBError(w, "save trivia answers", err, "quiz not found", "failed to save answers")
		return
	}

	writeTriviaQuiz(w, r, profileID, id, http.StatusOK)
}

func handleRetakeTrivia(w http.ResponseWriter, r *http.Request, profileID int64) {
	id, ok := pathID(w, r, "id", "quiz")
	if !ok {
		return
	}

	newID, err := database.RetakeTriviaQuiz(r.Context(), profileID, id)
	if err != nil {
		writeDBError(w, "retake trivia quiz", err, "quiz not found", "failed to retake quiz")
		return
	}

	writeTriviaQuiz(w, r, profileID, newID, http.StatusCreated)
}

func handleDeleteTrivia(w http.ResponseWriter, r *http.Request, profileID int64) {
	id, ok := pathID(w, r, "id", "quiz")
	if !ok {
		return
	}

	if err := database.DeleteTriviaQuiz(r.Context(), profileID, id); err != nil {
		writeDBError(w, "delete trivia quiz", err, "quiz not found", "failed to delete quiz")
		return
	}

	writeJSON(w, http.StatusOK, statusResponse("deleted"))
}
