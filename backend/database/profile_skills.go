package database

import (
	"context"
	"database/sql"
	"fmt"
)

type ProfileSkill struct {
	ID    int64  `json:"id"`
	Skill string `json:"skill"`
}

type AddSkillRequest struct {
	Skill string `json:"skill"`
}

func listSkills(ctx context.Context, db *sql.DB, profileID int64) ([]ProfileSkill, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, skill FROM profiles_skills WHERE profile_id = $1 ORDER BY skill`, profileID)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var skills []ProfileSkill

	for rows.Next() {
		var skill ProfileSkill

		if err := rows.Scan(&skill.ID, &skill.Skill); err != nil {
			return nil, err
		}

		skills = append(skills, skill)
	}

	return skills, rows.Err()
}

func AddSkill(ctx context.Context, profileID int64, req *AddSkillRequest) (int64, error) {
	if req.Skill == "" {
		return 0, fmt.Errorf("skill is required")
	}

	db, err := GetDb()

	if err != nil {
		return 0, err
	}

	var skillID int64

	err = db.QueryRowContext(ctx, insertStatements["profiles_skills"], profileID, req.Skill).Scan(&skillID)

	if err != nil {
		return 0, err
	}

	return skillID, nil
}

func DeleteSkill(ctx context.Context, profileID, skillID int64) error {
	db, err := GetDb()

	if err != nil {
		return err
	}

	result, err := db.ExecContext(ctx, `DELETE FROM profiles_skills WHERE id = $1 AND profile_id = $2`, skillID, profileID)

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
