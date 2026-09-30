package llm

import (
	"slices"
	"strings"
	"testing"
)

func validQuestion(text string) TriviaQuestion {
	return TriviaQuestion{
		Question:     text,
		Options:      []string{"A", "B", "C", "D"},
		CorrectIndex: 2,
		Explanation:  "Because.",
		Topic:        "Go",
	}
}

func TestSanitizeTriviaDropsMalformedQuestions(t *testing.T) {
	emptyQuestion := validQuestion("  ")
	threeOptions := validQuestion("three options")
	threeOptions.Options = []string{"A", "B", "C"}
	emptyOption := validQuestion("empty option")
	emptyOption.Options = []string{"A", " ", "C", "D"}
	duplicateOption := validQuestion("duplicate option")
	duplicateOption.Options = []string{"A", "b", "B", "D"}
	negativeIndex := validQuestion("negative index")
	negativeIndex.CorrectIndex = -1
	indexTooHigh := validQuestion("index too high")
	indexTooHigh.CorrectIndex = 4

	raw := []TriviaQuestion{
		emptyQuestion, threeOptions, emptyOption, duplicateOption, negativeIndex, indexTooHigh,
		validQuestion("  kept  "),
	}

	got := sanitizeTrivia(raw, 10)

	if len(got) != 1 || got[0].Question != "kept" {
		t.Fatalf("sanitizeTrivia() = %+v, want only the trimmed valid question", got)
	}
}

func TestSanitizeTriviaCapsAtCount(t *testing.T) {
	raw := []TriviaQuestion{validQuestion("one"), validQuestion("two"), validQuestion("three")}

	if got := sanitizeTrivia(raw, 2); len(got) != 2 {
		t.Errorf("len(sanitizeTrivia(3 questions, 2)) = %d, want 2", len(got))
	}
}

func TestShuffleOptionsKeepsCorrectAnswer(t *testing.T) {
	for range 50 {
		q := TriviaQuestion{Options: []string{"wrong1", "wrong2", "right", "wrong3"}, CorrectIndex: 2}
		shuffleOptions(&q)

		if q.Options[q.CorrectIndex] != "right" {
			t.Fatalf("after shuffle Options[%d] = %q, want %q", q.CorrectIndex, q.Options[q.CorrectIndex], "right")
		}
	}
}

func TestTriviaPromptUsesSource(t *testing.T) {
	resumePrompt := triviaPrompt(TriviaRequest{
		Resume:     &ResumeProfile{Skills: []string{"Kubernetes", "Rust"}},
		Count:      5,
		Difficulty: "hard",
	})

	for _, want := range []string{"Kubernetes, Rust", "exactly 5 hard-difficulty"} {
		if !strings.Contains(resumePrompt, want) {
			t.Errorf("resume prompt missing %q, got:\n%s", want, resumePrompt)
		}
	}

	topicPrompt := triviaPrompt(TriviaRequest{Prompt: "React hooks", Count: 10, Difficulty: "easy"})

	if !strings.Contains(topicPrompt, "<topic>React hooks</topic>") {
		t.Errorf("topic prompt missing delimited topic, got:\n%s", topicPrompt)
	}
}

func TestTriviaSchemaRequiresQuestionFields(t *testing.T) {
	questions := triviaSchema()["properties"].(map[string]any)["questions"].(map[string]any)
	required := questions["items"].(map[string]any)["required"].([]string)

	for _, field := range []string{"question", "options", "correct_index", "explanation", "topic"} {
		if !slices.Contains(required, field) {
			t.Errorf("triviaSchema() question required missing %q", field)
		}
	}
}
