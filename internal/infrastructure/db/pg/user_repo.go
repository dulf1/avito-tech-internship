package pg

import (
	"context"
	"database/sql"
	"errors"

	"prservice/internal/domain"
	"prservice/internal/domain/user"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) UpsertInTeam(ctx context.Context, teamName string, members []user.User) error {
	for _, u := range members {
		if _, err := exec(ctx, r.db,
			`INSERT INTO users (user_id, username, team_name, is_active)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (user_id) DO UPDATE
			   SET username = EXCLUDED.username,
			       team_name = EXCLUDED.team_name,
			       is_active = EXCLUDED.is_active`,
			u.ID, u.Username, teamName, u.IsActive,
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *UserRepository) SetActive(ctx context.Context, userID string, isActive bool) (user.User, error) {
	var u user.User
	err := queryRow(ctx, r.db,
		`UPDATE users
		   	SET is_active = $2
		 	WHERE user_id = $1
		 	RETURNING user_id, username, team_name, is_active`,
		userID, isActive,
	).Scan(&u.ID, &u.Username, &u.TeamName, &u.IsActive)

	if errors.Is(err, sql.ErrNoRows) {
		return user.User{}, &domain.DomainError{
			Code:       domain.ErrorCodeNotFound,
			Message:    "user not found",
			HTTPStatus: 404,
		}
	}
	if err != nil {
		return user.User{}, err
	}

	return u, nil
}

func (r *UserRepository) GetByID(ctx context.Context, userID string) (user.User, error) {
	var u user.User
	err := queryRow(ctx, r.db,
		`SELECT user_id, username, team_name, is_active
		   	FROM users
		  	WHERE user_id = $1`,
		userID,
	).Scan(&u.ID, &u.Username, &u.TeamName, &u.IsActive)

	if errors.Is(err, sql.ErrNoRows) {
		return user.User{}, &domain.DomainError{
			Code:       domain.ErrorCodeNotFound,
			Message:    "user not found",
			HTTPStatus: 404,
		}
	}
	if err != nil {
		return user.User{}, err
	}

	return u, nil
}

func (r *UserRepository) GetActiveTeamMembersExcept(ctx context.Context, teamName, excludeUserID string) ([]user.User, error) {
	rows, err := query(ctx, r.db,
		`SELECT user_id, username, team_name, is_active
		   	FROM users
		  	WHERE team_name = $1
		    AND is_active = TRUE
		    AND user_id <> $2`,
		teamName, excludeUserID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []user.User
	for rows.Next() {
		var u user.User
		if err := rows.Scan(&u.ID, &u.Username, &u.TeamName, &u.IsActive); err != nil {
			return nil, err
		}
		res = append(res, u)
	}
	return res, rows.Err()
}
