package pr

import "context"

type Repository interface {
	CreateWithReviewers(ctx context.Context, pr PullRequest) (PullRequest, error)
	LockByID(ctx context.Context, id string) (PullRequest, error)
	UpdateStatusMerged(ctx context.Context, id string) (PullRequest, error)
	GetReviewers(ctx context.Context, prID string) ([]string, error)
	SetReviewers(ctx context.Context, prID string, reviewerIDs []string) error
	UserIsReviewer(ctx context.Context, prID, userID string) (bool, error)
	GetUserPRs(ctx context.Context, userID string) ([]PullRequestShort, error)
}
