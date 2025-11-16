package pg

import (
	"context"
	"database/sql"
	"errors"

	"prservice/internal/domain"
	"prservice/internal/domain/team"
)

type TeamRepository struct {
	db *sql.DB
}

func NewTeamRepository(db *sql.DB) *TeamRepository {
	return &TeamRepository{db: db}
}

func (r *TeamRepository) Exists(ctx context.Context, name string) (bool, error) {
	var exists bool
	err := queryRow(ctx, r.db,
		`SELECT EXISTS(SELECT 1 FROM teams WHERE team_name = $1)`,
		name,
	).Scan(&exists)
	return exists, err
}

func (r *TeamRepository) Create(ctx context.Context, name string) error {
	_, err := exec(ctx, r.db,
		`INSERT INTO teams (team_name) VALUES ($1)`,
		name,
	)
	return err
}

func (r *TeamRepository) GetWithMembers(ctx context.Context, name string) (team.Team, error) {
	var t team.Team
	err := queryRow(ctx, r.db,
		`SELECT team_name FROM teams WHERE team_name = $1`,
		name,
	).Scan(&t.Name)

	if errors.Is(err, sql.ErrNoRows) {
		return team.Team{}, &domain.DomainError{
			Code:       domain.ErrorCodeNotFound,
			Message:    "team not found",
			HTTPStatus: 404,
		}
	}
	if err != nil {
		return team.Team{}, err
	}

	rows, err := query(ctx, r.db,
		`SELECT user_id, username, is_active
		   FROM users
		  WHERE team_name = $1
		  ORDER BY user_id`,
		name,
	)
	if err != nil {
		return team.Team{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var m team.Member
		if err := rows.Scan(&m.ID, &m.Username, &m.IsActive); err != nil {
			return team.Team{}, err
		}
		t.Members = append(t.Members, m)
	}
	if err := rows.Err(); err != nil {
		return team.Team{}, err
	}
	return t, nil
}
