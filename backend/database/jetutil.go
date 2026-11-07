package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	pg "github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/qrm"
)

// queryOne runs stmt into a single-row dest, reporting no row as sql.ErrNoRows like database/sql.
func queryOne(ctx context.Context, q qrm.Queryable, stmt pg.Statement, dest any) error {
	err := stmt.QueryContext(ctx, q, dest)
	if errors.Is(err, qrm.ErrNoRows) {
		return sql.ErrNoRows
	}

	return err
}

// exec runs stmt itself, since jet's ExecContext drops a RowsAffected error for its logger.
func exec(ctx context.Context, q qrm.Executable, stmt pg.Statement) (sql.Result, error) {
	query, args := stmt.Sql()

	return q.ExecContext(ctx, query, args...)
}

// execOne runs stmt and reports sql.ErrNoRows when it touched no row.
func execOne(ctx context.Context, stmt pg.Statement) error {
	result, err := exec(ctx, db(), stmt)
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

// setExcluded is the usual upsert update, "col = excluded.col" for each of cols except the conflict key.
func setExcluded(cols, excluded pg.ColumnList, key ...pg.Column) pg.ColumnAssigment {
	byName := make(map[string]pg.Column, len(excluded))
	for _, col := range excluded {
		byName[col.Name()] = col
	}

	isKey := make(map[string]bool, len(key))
	for _, col := range key {
		isKey[col.Name()] = true
	}

	var set, from pg.ColumnList
	for _, col := range cols {
		if !isKey[col.Name()] {
			set = append(set, col)
			from = append(from, byName[col.Name()])
		}
	}

	return set.SET(from)
}

// scalar is a single-value subquery used as an expression; jet leaves those unparenthesized.
func scalar(sub pg.SelectStatement) pg.Expression {
	return pg.CustomExpression(pg.Token("("), sub, pg.Token(")"))
}

// eqAny is "col = ANY(values)", one array param however many values there are.
func eqAny(col pg.Expression, values any, arrayType string) pg.BoolExpression {
	return pg.BoolExp(pg.CustomExpression(col, pg.Token("= ANY("), arg(values, arrayType), pg.Token(")")))
}

// arg binds a param jet has no literal for, such as an array or a pgvector, cast to sqlType.
func arg(value any, sqlType string) pg.Expression {
	return pg.Raw("#arg::"+sqlType, pg.RawArgs{"#arg": value})
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}

	return *p
}

// decodeJSON unmarshals a nullable JSON column into dest, leaving dest alone on NULL.
func decodeJSON(raw *string, dest any, what string) error {
	if raw == nil {
		return nil
	}

	if err := json.Unmarshal([]byte(*raw), dest); err != nil {
		return fmt.Errorf("decode %s: %w", what, err)
	}

	return nil
}

// encodeJSON marshals v for a JSON column; the plain structs stored there can't fail to marshal.
func encodeJSON(v any) string {
	encoded, _ := json.Marshal(v)

	return string(encoded)
}

// formatDate renders a nullable DATE column as the API's yyyy-mm-dd string.
func formatDate(t *time.Time) *string {
	if t == nil {
		return nil
	}

	formatted := t.Format(dateLayout)

	return &formatted
}
