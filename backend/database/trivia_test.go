package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"main/llm"
)

func TestTriviaQuizLifecycle(t *testing.T) {
	newTestDB(t)
	ctx := context.Background()
	profileID := newTestProfile(t)
	otherProfileID, err := CreateProfile(&ProfileRequest{Email: "other@example.com", Password: "securepassword123"})
	if err != nil {
		t.Fatalf("create other profile: %v", err)
	}

	questions := []llm.TriviaQuestion{
		{Question: "Q1", Options: []string{"a", "b", "c", "d"}, CorrectIndex: 0},
		{Question: "Q2", Options: []string{"a", "b", "c", "d"}, CorrectIndex: 3},
	}

	id, err := CreateTriviaQuiz(ctx, profileID, "prompt", "Go", "easy", questions)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := GetTriviaQuiz(ctx, otherProfileID, id); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetTriviaQuiz as other profile err = %v, want sql.ErrNoRows", err)
	}

	if err := SaveTriviaAnswers(ctx, profileID, id, []int{0}); err != nil {
		t.Fatalf("save partial: %v", err)
	}

	quiz, err := GetTriviaQuiz(ctx, profileID, id)
	if err != nil {
		t.Fatalf("get after partial: %v", err)
	}
	if quiz.Score != 1 || quiz.CompletedAt != nil {
		t.Errorf("partial score=%d completed=%v, want 1/nil", quiz.Score, quiz.CompletedAt)
	}

	if err := SaveTriviaAnswers(ctx, profileID, id, []int{0, 1, 2}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("too many answers err = %v, want ErrInvalidInput", err)
	}
	if err := SaveTriviaAnswers(ctx, profileID, id, []int{4}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("out of range answer err = %v, want ErrInvalidInput", err)
	}

	if err := SaveTriviaAnswers(ctx, profileID, id, []int{0, 1}); err != nil {
		t.Fatalf("save complete: %v", err)
	}

	quiz, err = GetTriviaQuiz(ctx, profileID, id)
	if err != nil {
		t.Fatalf("get after complete: %v", err)
	}
	if quiz.Score != 1 || quiz.CompletedAt == nil {
		t.Errorf("complete score=%d completed=%v, want 1/set", quiz.Score, quiz.CompletedAt)
	}

	retakeID, err := RetakeTriviaQuiz(ctx, profileID, id)
	if err != nil {
		t.Fatalf("retake: %v", err)
	}

	summaries, err := ListTriviaQuizzes(ctx, profileID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(summaries) != 2 || summaries[0].ID != retakeID || summaries[0].AnsweredCount != 0 || summaries[1].Score != 1 {
		t.Errorf("list = %+v, want fresh retake first and original score kept", summaries)
	}

	if err := DeleteTriviaQuiz(ctx, otherProfileID, id); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("delete as other profile err = %v, want sql.ErrNoRows", err)
	}
	if err := DeleteTriviaQuiz(ctx, profileID, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := GetTriviaQuiz(ctx, profileID, id); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("get after delete err = %v, want sql.ErrNoRows", err)
	}
}
