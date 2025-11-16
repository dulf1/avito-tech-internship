package pr

import "time"

type Status string

const (
	StatusOpen   Status = "OPEN"
	StatusMerged Status = "MERGED"
)

type PullRequest struct {
	ID                string
	Name              string
	AuthorID          string
	Status            Status
	AssignedReviewers []string
	CreatedAt         *time.Time
	MergedAt          *time.Time
}

type PullRequestShort struct {
	ID       string
	Name     string
	AuthorID string
	Status   Status
}
