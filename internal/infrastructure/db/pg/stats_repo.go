package pg

import (
	"context"
	"database/sql"

	"prservice/internal/domain/stats"
)

type StatsRepository struct {
	db *sql.DB
}

func NewStatsRepository(db *sql.DB) *StatsRepository {
	return &StatsRepository{db: db}
}

func (r *StatsRepository) GetUserAssignmentStats(ctx context.Context, teamName *string) ([]stats.UserAssignmentStat, error) {
	const q = `
	SELECT u.user_id, COUNT(prr.pull_request_id) AS assigned_total, 
	COUNT(prr.pull_request_id) FILTER (WHERE pr.status = 'OPEN') AS assigned_open,
    COUNT(prr.pull_request_id) FILTER (WHERE pr.status = 'MERGED') AS assigned_merged
	FROM users u LEFT JOIN pull_request_reviewers prr ON u.user_id = prr.user_id
	LEFT JOIN pull_requests pr ON prr.pull_request_id = pr.pull_request_id
	WHERE ($1::text IS NULL OR u.team_name = $1::text)
	GROUP BY u.user_id
	ORDER BY u.user_id;`

	var arg interface{}
	if teamName != nil {
		arg = *teamName
	} else {
		arg = nil
	}

	rows, err := query(ctx, r.db, q, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []stats.UserAssignmentStat
	for rows.Next() {
		var s stats.UserAssignmentStat
		if err := rows.Scan(
			&s.UserID,
			&s.AssignedTotal,
			&s.AssignedOpen,
			&s.AssignedMerged,
		); err != nil {
			return nil, err
		}
		res = append(res, s)
	}

	return res, rows.Err()
}

func (r *StatsRepository) GetPRAssignmentStats(ctx context.Context) ([]stats.PRAssignmentStat, error) {
	const q = `
	SELECT pr.pull_request_id, COUNT(prr.user_id) AS reviewer_count 
	FROM pull_requests pr LEFT JOIN pull_request_reviewers prr ON pr.pull_request_id = prr.pull_request_id
	GROUP BY pr.pull_request_id
	ORDER BY pr.pull_request_id;`

	rows, err := query(ctx, r.db, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []stats.PRAssignmentStat
	for rows.Next() {
		var s stats.PRAssignmentStat
		if err := rows.Scan(
			&s.PullRequestID,
			&s.ReviewerCount,
		); err != nil {
			return nil, err
		}
		res = append(res, s)
	}

	return res, rows.Err()
}
