package user

import (
	"context"

	"prservice/internal/domain"
)

type Service interface {
	SetUserActive(ctx context.Context, userID string, isActive bool) (User, error)
}

type service struct {
	uow    domain.UnitOfWork
	users  Repository
	events domain.EventBus
}

func NewService(uow domain.UnitOfWork, users Repository, events domain.EventBus) Service {
	return &service{
		uow:    uow,
		users:  users,
		events: events,
	}
}

func (s *service) SetUserActive(ctx context.Context, userID string, isActive bool) (User, error) {
	var res User

	err := s.uow.WithinTx(ctx, func(ctx context.Context) error {
		u, err := s.users.SetActive(ctx, userID, isActive)
		if err != nil {
			return err
		}
		res = u

		if s.events != nil {
			s.events.Publish(ctx, domain.Event{
				Type: "user.set_active",
				Payload: map[string]any{
					"user_id":   u.ID,
					"is_active": u.IsActive,
				},
			})
		}
		return nil
	})

	return res, err
}
