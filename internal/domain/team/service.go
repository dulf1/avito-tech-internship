package team

import (
	"context"
	"net/http"

	"prservice/internal/domain"
	"prservice/internal/domain/user"
)

type Service interface {
	AddTeam(ctx context.Context, team Team) (Team, error)
	GetTeam(ctx context.Context, name string) (Team, error)
}

type service struct {
	uow    domain.UnitOfWork
	teams  Repository
	users  user.Repository
	events domain.EventBus
}

func NewService(
	uow domain.UnitOfWork,
	teams Repository,
	users user.Repository,
	events domain.EventBus,
) Service {
	return &service{
		uow:    uow,
		teams:  teams,
		users:  users,
		events: events,
	}
}

func (s *service) AddTeam(ctx context.Context, t Team) (Team, error) {
	var result Team

	err := s.uow.WithinTx(ctx, func(ctx context.Context) error {
		exists, err := s.teams.Exists(ctx, t.Name)
		if err != nil {
			return err
		}
		if exists {
			return &domain.DomainError{
				Code:       domain.ErrorCodeTeamExists,
				Message:    "team_name already exists",
				HTTPStatus: http.StatusBadRequest,
			}
		}

		if err := s.teams.Create(ctx, t.Name); err != nil {
			return err
		}

		users := make([]user.User, 0, len(t.Members))
		for _, m := range t.Members {
			users = append(users, user.User{
				ID:       m.ID,
				Username: m.Username,
				TeamName: t.Name,
				IsActive: m.IsActive,
			})
		}

		if err := s.users.UpsertInTeam(ctx, t.Name, users); err != nil {
			return err
		}

		result = t

		if s.events != nil {
			s.events.Publish(ctx, domain.Event{
				Type: "team.created",
				Payload: map[string]any{
					"team_name": t.Name,
					"members":   len(t.Members),
				},
			})
		}
		return nil
	})

	return result, err
}

func (s *service) GetTeam(ctx context.Context, name string) (Team, error) {
	return s.teams.GetWithMembers(ctx, name)
}
