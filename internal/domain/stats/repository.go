package stats

import "context"

type Repository interface {
	GetUserAssignmentStats(ctx context.Context, teamName *string) ([]UserAssignmentStat, error)
	GetPRAssignmentStats(ctx context.Context) ([]PRAssignmentStat, error)
}
