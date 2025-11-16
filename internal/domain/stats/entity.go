package stats

type UserAssignmentStat struct {
	UserID         string
	AssignedTotal  int
	AssignedOpen   int
	AssignedMerged int
}

type PRAssignmentStat struct {
	PullRequestID string
	ReviewerCount int
}
