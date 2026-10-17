package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"main/database"
	"main/llm"
)

func triviaJSON(n int) string {
	var questions []llm.TriviaQuestion
	for i := range n {
		questions = append(questions, llm.TriviaQuestion{
			Question:     fmt.Sprintf("Question %d?", i),
			Options:      []string{"A", "B", "C", "D"},
			CorrectIndex: 1,
			Explanation:  "Because.",
			Topic:        "Go",
		})
	}

	raw, _ := json.Marshal(map[string]any{"questions": questions})

	return string(raw)
}

func TestAPITriviaLifecycle(t *testing.T) {
	a := newAPI(t).signup("api-trivia@example.com")
	other := a.as().signup("api-trivia-other@example.com")
	fakeLLM(t, static(triviaJSON(5)))

	var quiz database.TriviaQuiz
	a.must(http.StatusCreated, "POST", "/api/trivia",
		map[string]any{"source": "prompt", "prompt": "  Go channels  ", "count": 5, "difficulty": "easy"}, &quiz)

	if quiz.ID == 0 || quiz.Topic != "Go channels" || quiz.Source != "prompt" || len(quiz.Questions) != 5 || quiz.CompletedAt != nil {
		t.Fatalf("quiz = %+v", quiz)
	}

	path := fmt.Sprintf("/api/trivia/%d", quiz.ID)

	correct := func(q llm.TriviaQuestion) int { return q.CorrectIndex }
	wrong := func(q llm.TriviaQuestion) int { return (q.CorrectIndex + 1) % len(q.Options) }

	// Answer two, one right.
	a.must(http.StatusOK, "PUT", path+"/answers",
		map[string]any{"answers": []int{correct(quiz.Questions[0]), wrong(quiz.Questions[1])}}, &quiz)
	if quiz.Score != 1 || len(quiz.Answers) != 2 || quiz.CompletedAt != nil {
		t.Errorf("partial quiz = %+v", quiz)
	}

	var all []int
	for _, q := range quiz.Questions {
		all = append(all, correct(q))
	}
	a.must(http.StatusOK, "PUT", path+"/answers", map[string]any{"answers": all}, &quiz)
	if quiz.Score != 5 || quiz.CompletedAt == nil {
		t.Errorf("finished quiz = %+v", quiz)
	}

	a.must(http.StatusBadRequest, "PUT", path+"/answers", map[string]any{"answers": append(all, 0)}, nil)
	a.must(http.StatusBadRequest, "PUT", path+"/answers", map[string]any{"answers": []int{9}}, nil)
	a.must(http.StatusBadRequest, "PUT", path+"/answers", "{", nil)

	var retake database.TriviaQuiz
	a.must(http.StatusCreated, "POST", path+"/retake", nil, &retake)
	if retake.ID == quiz.ID || len(retake.Answers) != 0 || retake.Score != 0 || len(retake.Questions) != 5 {
		t.Errorf("retake = %+v", retake)
	}

	var list []database.TriviaQuizSummary
	a.must(http.StatusOK, "GET", "/api/trivia", nil, &list)
	if len(list) != 2 {
		t.Errorf("list = %+v", list)
	}

	var otherList []database.TriviaQuizSummary
	other.must(http.StatusOK, "GET", "/api/trivia", nil, &otherList)
	if len(otherList) != 0 {
		t.Errorf("other profile sees %d quizzes", len(otherList))
	}

	other.must(http.StatusNotFound, "GET", path, nil, nil)
	other.must(http.StatusNotFound, "PUT", path+"/answers", map[string]any{"answers": []int{}}, nil)
	other.must(http.StatusNotFound, "POST", path+"/retake", nil, nil)
	other.must(http.StatusNotFound, "DELETE", path, nil, nil)

	for _, req := range [][2]string{{"GET", ""}, {"PUT", "/answers"}, {"POST", "/retake"}, {"DELETE", ""}} {
		a.must(http.StatusBadRequest, req[0], "/api/trivia/x"+req[1], map[string]any{"answers": []int{}}, nil)
	}

	a.must(http.StatusOK, "DELETE", path, nil, nil)
	a.must(http.StatusNotFound, "GET", path, nil, nil)
	a.must(http.StatusNotFound, "DELETE", path, nil, nil)
}

func TestAPITriviaValidation(t *testing.T) {
	a := newAPI(t).signup("api-trivia-validate@example.com")
	calls := fakeLLM(t, static(triviaJSON(5)))

	bad := []map[string]any{
		{"source": "prompt", "prompt": "Go", "count": 5, "difficulty": "easy"},
		{"source": "prompt", "prompt": strings.Repeat("x", 301), "count": 5, "difficulty": "easy"},
		{"source": "web", "prompt": "Go stuff", "count": 5, "difficulty": "easy"},
		{"source": "prompt", "prompt": "Go stuff", "count": 7, "difficulty": "easy"},
		{"source": "prompt", "prompt": "Go stuff", "count": 5, "difficulty": "insane"},
	}

	for _, body := range bad {
		a.must(http.StatusBadRequest, "POST", "/api/trivia", body, nil)
	}
	a.must(http.StatusBadRequest, "POST", "/api/trivia", "{", nil)

	// No active resume to quiz on.
	a.must(http.StatusNotFound, "POST", "/api/trivia", map[string]any{"source": "resume", "count": 5, "difficulty": "easy"}, nil)

	if n := calls.count("chat"); n != 0 {
		t.Errorf("invalid requests made %d LLM calls", n)
	}
}

func TestAPITriviaFromResume(t *testing.T) {
	a := newAPI(t).signup("api-trivia-resume@example.com")
	id := a.profile().ID

	resumeID, err := database.AddResume(t.Context(), id, "r.pdf", "application/pdf", []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	if err := database.SaveResumeExtractionResult(t.Context(), resumeID, &llm.ExtractedResume{FullName: "Secret Name", Skills: []string{"Rust"}}); err != nil {
		t.Fatal(err)
	}

	calls := fakeLLM(t, static(triviaJSON(10)))

	var quiz database.TriviaQuiz
	a.must(http.StatusCreated, "POST", "/api/trivia", map[string]any{"source": "resume", "prompt": "ignored", "count": 10, "difficulty": "hard"}, &quiz)

	if quiz.Topic != triviaResumeTopic || quiz.Source != "resume" || quiz.Difficulty != "hard" || len(quiz.Questions) != 10 {
		t.Errorf("quiz = %+v", quiz)
	}

	if prompt := calls.prompt(); !strings.Contains(prompt, "Rust") || strings.Contains(prompt, "Secret Name") {
		t.Errorf("prompt should list skills but not the name: %q", prompt)
	}
}

func TestAPITriviaUpstreamFailure(t *testing.T) {
	a := newAPI(t).signup("api-trivia-down@example.com")
	fakeLLM(t, static(""))

	a.must(http.StatusBadGateway, "POST", "/api/trivia", map[string]any{"source": "prompt", "prompt": "Go stuff", "count": 5, "difficulty": "easy"}, nil)

	var list []database.TriviaQuizSummary
	a.must(http.StatusOK, "GET", "/api/trivia", nil, &list)
	if len(list) != 0 {
		t.Errorf("failed generation saved %d quizzes", len(list))
	}
}
