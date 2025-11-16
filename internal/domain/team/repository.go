package team

import "context"

type Repository interface {
	Exists(ctx context.Context, name string) (bool, error)
	Create(ctx context.Context, name string) error
	GetWithMembers(ctx context.Context, name string) (Team, error)
}
