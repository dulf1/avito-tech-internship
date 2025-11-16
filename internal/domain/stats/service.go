package stats

import "context"

type Service interface {
	GetUserStats(ctx context.Context, teamName *string) ([]UserAssignmentStat, error)
	GetPRStats(ctx context.Context) ([]PRAssignmentStat, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetUserStats(ctx context.Context, teamName *string) ([]UserAssignmentStat, error) {
	return s.repo.GetUserAssignmentStats(ctx, teamName)
}

func (s *service) GetPRStats(ctx context.Context) ([]PRAssignmentStat, error) {
	return s.repo.GetPRAssignmentStats(ctx)
}
