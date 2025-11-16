package user

import "context"

type Repository interface {
	UpsertInTeam(ctx context.Context, teamName string, members []User) error
	SetActive(ctx context.Context, userID string, isActive bool) (User, error)
	GetByID(ctx context.Context, userID string) (User, error)
	GetActiveTeamMembersExcept(ctx context.Context, teamName, excludeUserID string) ([]User, error)
}
