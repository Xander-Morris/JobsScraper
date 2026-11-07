package database

import (
	"context"
	"time"

	pg "github.com/go-jet/jet/v2/postgres"

	"main/internal/gen/model"
	"main/internal/gen/table"
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

// ownedQuiz matches the quiz only when profileID owns it.
func ownedQuiz(id, profileID int64) pg.BoolExpression {
	quizzes := table.TriviaQuizzes

	return quizzes.ID.EQ(pg.Int(id)).AND(quizzes.ProfileID.EQ(pg.Int(profileID)))
}

// triviaQuizFresh are the columns a new attempt starts from; answers, score and timestamps take their defaults.
func triviaQuizFresh() pg.ColumnList {
	quizzes := table.TriviaQuizzes

	return pg.ColumnList{quizzes.ProfileID, quizzes.Source, quizzes.Topic, quizzes.Difficulty, quizzes.Questions}
}

func CreateTriviaQuiz(ctx context.Context, profileID int64, source, topic, difficulty string, questions []llm.TriviaQuestion) (int64, error) {
	row := model.TriviaQuizzes{
		ProfileID:  int32(profileID),
		Source:     source,
		Topic:      topic,
		Difficulty: difficulty,
		Questions:  encodeJSON(questions),
	}

	err := queryOne(ctx, db(), table.TriviaQuizzes.INSERT(triviaQuizFresh()).MODEL(row).RETURNING(table.TriviaQuizzes.ID), &row)

	return int64(row.ID), err
}

func ListTriviaQuizzes(ctx context.Context, profileID int64) ([]TriviaQuizSummary, error) {
	var rows []struct {
		model.TriviaQuizzes
		QuestionCount int
		AnsweredCount int
	}

	quizzes := table.TriviaQuizzes
	stmt := pg.SELECT(
		quizzes.AllColumns.Except(quizzes.Questions, quizzes.Answers),
		pg.Func("jsonb_array_length", quizzes.Questions).AS("question_count"),
		pg.Func("jsonb_array_length", quizzes.Answers).AS("answered_count"),
	).FROM(quizzes).
		WHERE(quizzes.ProfileID.EQ(pg.Int(profileID))).
		ORDER_BY(quizzes.CreatedAt.DESC(), quizzes.ID.DESC()).
		LIMIT(triviaHistoryLimit)

	if err := stmt.QueryContext(ctx, db(), &rows); err != nil {
		return nil, err
	}

	summaries := []TriviaQuizSummary{}

	for _, row := range rows {
		summaries = append(summaries, TriviaQuizSummary{
			ID:            int64(row.ID),
			Source:        row.Source,
			Topic:         row.Topic,
			Difficulty:    row.Difficulty,
			QuestionCount: row.QuestionCount,
			AnsweredCount: row.AnsweredCount,
			Score:         int(row.Score),
			CompletedAt:   row.CompletedAt,
			CreatedAt:     row.CreatedAt,
		})
	}

	return summaries, nil
}

func GetTriviaQuiz(ctx context.Context, profileID, id int64) (TriviaQuiz, error) {
	var row model.TriviaQuizzes

	quizzes := table.TriviaQuizzes

	if err := queryOne(ctx, db(), pg.SELECT(quizzes.AllColumns).FROM(quizzes).WHERE(ownedQuiz(id, profileID)), &row); err != nil {
		return TriviaQuiz{}, err
	}

	quiz := TriviaQuiz{
		ID:          int64(row.ID),
		Source:      row.Source,
		Topic:       row.Topic,
		Difficulty:  row.Difficulty,
		Score:       int(row.Score),
		CompletedAt: row.CompletedAt,
		CreatedAt:   row.CreatedAt,
	}

	if err := decodeJSON(&row.Questions, &quiz.Questions, "trivia questions"); err != nil {
		return TriviaQuiz{}, err
	}

	if err := decodeJSON(&row.Answers, &quiz.Answers, "trivia answers"); err != nil {
		return TriviaQuiz{}, err
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

	quizzes := table.TriviaQuizzes
	_, err = exec(ctx, db(), quizzes.UPDATE(quizzes.Answers, quizzes.Score, quizzes.CompletedAt).
		MODEL(model.TriviaQuizzes{Answers: encodeJSON(answers), Score: int32(score), CompletedAt: completedAt}).
		WHERE(ownedQuiz(id, profileID)))

	return err
}

// RetakeTriviaQuiz copies a quiz's questions into a fresh attempt, keeping the old score in history.
func RetakeTriviaQuiz(ctx context.Context, profileID, id int64) (int64, error) {
	var row model.TriviaQuizzes

	quizzes := table.TriviaQuizzes
	fresh := triviaQuizFresh()
	stmt := quizzes.INSERT(fresh).
		QUERY(pg.SELECT(fresh).FROM(quizzes).WHERE(ownedQuiz(id, profileID))).
		RETURNING(quizzes.ID)

	err := queryOne(ctx, db(), stmt, &row)

	return int64(row.ID), err
}

func DeleteTriviaQuiz(ctx context.Context, profileID, id int64) error {
	return execOne(ctx, table.TriviaQuizzes.DELETE().WHERE(ownedQuiz(id, profileID)))
}
