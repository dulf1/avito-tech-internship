package dto

type UserAssignmentStat struct {
	UserID         string `json:"user_id"`
	AssignedTotal  int    `json:"assigned_total"`
	AssignedOpen   int    `json:"assigned_open"`
	AssignedMerged int    `json:"assigned_merged"`
}

type PRAssignmentStat struct {
	PullRequestID string `json:"pull_request_id"`
	ReviewerCount int    `json:"reviewer_count"`
}

type StatsResponse struct {
	PerUser []UserAssignmentStat `json:"per_user,omitempty"`
	PerPR   []PRAssignmentStat   `json:"per_pr,omitempty"`
}
