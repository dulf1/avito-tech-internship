package pg

import (
	"context"
	"database/sql"
	"errors"

	"prservice/internal/domain"
	"prservice/internal/domain/pr"
)

type PRRepository struct {
	db *sql.DB
}

func NewPRRepository(db *sql.DB) *PRRepository {
	return &PRRepository{db: db}
}

func (r *PRRepository) CreateWithReviewers(ctx context.Context, p pr.PullRequest) (pr.PullRequest, error) {
	var exists bool
	if err := queryRow(ctx, r.db,
		`SELECT EXISTS(SELECT 1 FROM pull_requests WHERE pull_request_id = $1)`,
		p.ID,
	).Scan(&exists); err != nil {
		return pr.PullRequest{}, err
	}
	if exists {
		return pr.PullRequest{}, &domain.DomainError{
			Code:       domain.ErrorCodePRExists,
			Message:    "PR id already exists",
			HTTPStatus: 409,
		}
	}

	var createdAt sql.NullTime
	if err := queryRow(ctx, r.db,
		`INSERT INTO pull_requests (pull_request_id, pull_request_name, author_id, status)
		 VALUES ($1, $2, $3, $4)
		 RETURNING created_at`,
		p.ID, p.Name, p.AuthorID, string(p.Status),
	).Scan(&createdAt); err != nil {
		return pr.PullRequest{}, err
	}
	if createdAt.Valid {
		t := createdAt.Time
		p.CreatedAt = &t
	}

	for _, uid := range p.AssignedReviewers {
		if _, err := exec(ctx, r.db,
			`INSERT INTO pull_request_reviewers (pull_request_id, user_id)
			 VALUES ($1, $2)`,
			p.ID, uid,
		); err != nil {
			return pr.PullRequest{}, err
		}
	}
	return p, nil
}

func (r *PRRepository) LockByID(ctx context.Context, id string) (pr.PullRequest, error) {
	var p pr.PullRequest
	var status string
	var createdAt, mergedAt sql.NullTime

	err := queryRow(ctx, r.db,
		`SELECT pull_request_name, author_id, status, created_at, merged_at
		   FROM pull_requests
		  WHERE pull_request_id = $1
		  FOR UPDATE`,
		id,
	).Scan(&p.Name, &p.AuthorID, &status, &createdAt, &mergedAt)

	if errors.Is(err, sql.ErrNoRows) {
		return pr.PullRequest{}, &domain.DomainError{
			Code:       domain.ErrorCodeNotFound,
			Message:    "pull request not found",
			HTTPStatus: 404,
		}
	}
	if err != nil {
		return pr.PullRequest{}, err
	}

	p.ID = id
	p.Status = pr.Status(status)
	if createdAt.Valid {
		t := createdAt.Time
		p.CreatedAt = &t
	}
	if mergedAt.Valid {
		t := mergedAt.Time
		p.MergedAt = &t
	}

	return p, nil
}

func (r *PRRepository) UpdateStatusMerged(ctx context.Context, id string) (pr.PullRequest, error) {
	var p pr.PullRequest
	var status string
	var createdAt, mergedAt sql.NullTime

	err := queryRow(ctx, r.db,
		`UPDATE pull_requests
		   SET status = 'MERGED',
		       merged_at = NOW()
		 WHERE pull_request_id = $1
		 RETURNING pull_request_name, author_id, status, created_at, merged_at`,
		id,
	).Scan(&p.Name, &p.AuthorID, &status, &createdAt, &mergedAt)

	if err != nil {
		return pr.PullRequest{}, err
	}

	p.ID = id
	p.Status = pr.Status(status)
	if createdAt.Valid {
		t := createdAt.Time
		p.CreatedAt = &t
	}
	if mergedAt.Valid {
		t := mergedAt.Time
		p.MergedAt = &t
	}

	return p, nil
}

func (r *PRRepository) GetReviewers(ctx context.Context, prID string) ([]string, error) {
	rows, err := query(ctx, r.db,
		`SELECT user_id
		   FROM pull_request_reviewers
		  WHERE pull_request_id = $1
		  ORDER BY user_id`,
		prID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		res = append(res, id)
	}
	return res, rows.Err()
}

func (r *PRRepository) SetReviewers(ctx context.Context, prID string, reviewerIDs []string) error {
	if _, err := exec(ctx, r.db,
		`DELETE FROM pull_request_reviewers WHERE pull_request_id = $1`,
		prID,
	); err != nil {
		return err
	}
	for _, uid := range reviewerIDs {
		if _, err := exec(ctx, r.db,
			`INSERT INTO pull_request_reviewers (pull_request_id, user_id)
			 VALUES ($1, $2)`,
			prID, uid,
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *PRRepository) UserIsReviewer(ctx context.Context, prID, userID string) (bool, error) {
	var exists bool
	err := queryRow(ctx, r.db,
		`SELECT EXISTS(
			SELECT 1
			  FROM pull_request_reviewers
			 WHERE pull_request_id = $1
			   AND user_id = $2
		)`,
		prID, userID,
	).Scan(&exists)

	return exists, err
}

func (r *PRRepository) GetUserPRs(ctx context.Context, userID string) ([]pr.PullRequestShort, error) {
	rows, err := query(ctx, r.db,
		`SELECT pr.pull_request_id, pr.pull_request_name, pr.author_id, pr.status
		   	FROM pull_requests pr
		   	JOIN pull_request_reviewers prr
			ON pr.pull_request_id = prr.pull_request_id
		  	WHERE prr.user_id = $1
		  	ORDER BY pr.pull_request_id`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []pr.PullRequestShort
	for rows.Next() {
		var s pr.PullRequestShort
		var status string
		if err := rows.Scan(&s.ID, &s.Name, &s.AuthorID, &status); err != nil {
			return nil, err
		}
		s.Status = pr.Status(status)
		res = append(res, s)
	}
	return res, rows.Err()
}
