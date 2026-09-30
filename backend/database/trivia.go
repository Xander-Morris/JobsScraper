package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"main/llm"
)

const triviaHistoryLimit = 50

type TriviaQuiz struct {
	ID          int64                `json:"id"`
	Source      string               `json:"source"`
	Topic       string               `json:"topic"`
	Difficulty  string               `json:"difficulty"`
	Questions   []llm.TriviaQuestion `json:"questions"`
	Answers     []int                `json:"answers"`
	Score       int                  `json:"score"`
	CompletedAt *time.Time           `json:"completed_at"`
	CreatedAt   time.Time            `json:"created_at"`
}

type TriviaQuizSummary struct {
	ID            int64      `json:"id"`
	Source        string     `json:"source"`
	Topic         string     `json:"topic"`
	Difficulty    string     `json:"difficulty"`
	QuestionCount int        `json:"question_count"`
	AnsweredCount int        `json:"answered_count"`
	Score         int        `json:"score"`
	CompletedAt   *time.Time `json:"completed_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

func CreateTriviaQuiz(ctx context.Context, profileID int64, source, topic, difficulty string, questions []llm.TriviaQuestion) (int64, error) {
	encoded, err := json.Marshal(questions)
	if err != nil {
		return 0, fmt.Errorf("encode trivia questions: %w", err)
	}

	var id int64
	err = db().QueryRowContext(ctx, `INSERT INTO trivia_quizzes (profile_id, source, topic, difficulty, questions)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		profileID, source, topic, difficulty, string(encoded)).Scan(&id)

	return id, err
}

func ListTriviaQuizzes(ctx context.Context, profileID int64) ([]TriviaQuizSummary, error) {
	rows, err := db().QueryContext(ctx, `SELECT id, source, topic, difficulty,
			jsonb_array_length(questions), jsonb_array_length(answers), score, completed_at, created_at
		FROM trivia_quizzes WHERE profile_id = $1
		ORDER BY created_at DESC, id DESC LIMIT $2`, profileID, triviaHistoryLimit)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	quizzes := []TriviaQuizSummary{}

	for rows.Next() {
		var q TriviaQuizSummary

		if err := rows.Scan(&q.ID, &q.Source, &q.Topic, &q.Difficulty, &q.QuestionCount, &q.AnsweredCount,
			&q.Score, &q.CompletedAt, &q.CreatedAt); err != nil {
			return nil, err
		}

		quizzes = append(quizzes, q)
	}

	return quizzes, rows.Err()
}

func GetTriviaQuiz(ctx context.Context, profileID, id int64) (TriviaQuiz, error) {
	var quiz TriviaQuiz
	var questions, answers string

	err := db().QueryRowContext(ctx, `SELECT id, source, topic, difficulty, questions, answers, score, completed_at, created_at
		FROM trivia_quizzes WHERE id = $1 AND profile_id = $2`, id, profileID).Scan(
		&quiz.ID, &quiz.Source, &quiz.Topic, &quiz.Difficulty, &questions, &answers, &quiz.Score, &quiz.CompletedAt, &quiz.CreatedAt,
	)

	if err != nil {
		return TriviaQuiz{}, err
	}

	if err := json.Unmarshal([]byte(questions), &quiz.Questions); err != nil {
		return TriviaQuiz{}, fmt.Errorf("decode trivia questions: %w", err)
	}

	if err := json.Unmarshal([]byte(answers), &quiz.Answers); err != nil {
		return TriviaQuiz{}, fmt.Errorf("decode trivia answers: %w", err)
	}

	return quiz, nil
}

// SaveTriviaAnswers replaces the answers so far and scores them against the stored questions.
func SaveTriviaAnswers(ctx context.Context, profileID, id int64, answers []int) error {
	quiz, err := GetTriviaQuiz(ctx, profileID, id)
	if err != nil {
		return err
	}

	if len(answers) > len(quiz.Questions) {
		return invalidInput("quiz has only %d questions", len(quiz.Questions))
	}

	score := 0

	for i, answer := range answers {
		question := quiz.Questions[i]

		if answer < 0 || answer >= len(question.Options) {
			return invalidInput("answer %d is out of range", i+1)
		}

		if answer == question.CorrectIndex {
			score++
		}
	}

	var completedAt *time.Time
	if len(answers) == len(quiz.Questions) {
		now := time.Now()
		completedAt = &now
	}

	encoded, err := json.Marshal(answers)
	if err != nil {
		return fmt.Errorf("encode trivia answers: %w", err)
	}

	_, err = db().ExecContext(ctx, `UPDATE trivia_quizzes SET answers = $1, score = $2, completed_at = $3
		WHERE id = $4 AND profile_id = $5`, string(encoded), score, completedAt, id, profileID)

	return err
}

// RetakeTriviaQuiz copies a quiz's questions into a fresh attempt, keeping the old score in history.
func RetakeTriviaQuiz(ctx context.Context, profileID, id int64) (int64, error) {
	var newID int64

	err := db().QueryRowContext(ctx, `INSERT INTO trivia_quizzes (profile_id, source, topic, difficulty, questions)
		SELECT profile_id, source, topic, difficulty, questions FROM trivia_quizzes WHERE id = $1 AND profile_id = $2
		RETURNING id`, id, profileID).Scan(&newID)

	return newID, err
}

func DeleteTriviaQuiz(ctx context.Context, profileID, id int64) error {
	result, err := db().ExecContext(ctx, `DELETE FROM trivia_quizzes WHERE id = $1 AND profile_id = $2`, id, profileID)
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
