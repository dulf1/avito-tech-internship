package user_test

import (
	"context"
	"errors"
	"testing"

	"prservice/internal/domain"
	"prservice/internal/domain/user"
)

type uowStub struct{}

func (uowStub) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type eventBusFake struct{ events []domain.Event }

func (e *eventBusFake) Publish(ctx context.Context, ev domain.Event) { e.events = append(e.events, ev) }

type userRepoFake struct{ byID map[string]user.User }

func newUserRepoFake() *userRepoFake { return &userRepoFake{byID: map[string]user.User{}} }

func (r *userRepoFake) UpsertInTeam(ctx context.Context, teamName string, members []user.User) error {
	return nil
}
func (r *userRepoFake) GetActiveTeamMembersExcept(ctx context.Context, teamName, excludeUserID string) ([]user.User, error) {
	return nil, nil
}
func (r *userRepoFake) GetByID(ctx context.Context, userID string) (user.User, error) {
	u, ok := r.byID[userID]
	if !ok {
		return user.User{}, &domain.DomainError{Code: domain.ErrorCodeNotFound, Message: "user not found", HTTPStatus: 404}
	}
	return u, nil
}
func (r *userRepoFake) SetActive(ctx context.Context, userID string, isActive bool) (user.User, error) {
	u, ok := r.byID[userID]
	if !ok {
		return user.User{}, &domain.DomainError{Code: domain.ErrorCodeNotFound, Message: "user not found", HTTPStatus: 404}
	}
	u.IsActive = isActive
	r.byID[userID] = u
	return u, nil
}

func TestSetUserActive(t *testing.T) {
	uow := uowStub{}
	repo := newUserRepoFake()
	events := &eventBusFake{}

	repo.byID["u1"] = user.User{ID: "u1", Username: "Alice", TeamName: "backend", IsActive: false}

	svc := user.NewService(uow, repo, events)

	u, err := svc.SetUserActive(context.Background(), "u1", true)
	if err != nil {
		t.Fatalf("SetUserActive: %v", err)
	}
	if !u.IsActive {
		t.Fatalf("user should be active")
	}
	if len(events.events) != 1 || events.events[0].Type != "user.set_active" {
		t.Fatalf("expected user.set_active event, got %+v", events.events)
	}
}

func TestSetUserActive_NotFound(t *testing.T) {
	uow := uowStub{}
	repo := newUserRepoFake()
	events := &eventBusFake{}

	svc := user.NewService(uow, repo, events)
	_, err := svc.SetUserActive(context.Background(), "missing", true)
	var de *domain.DomainError
	if err == nil || !errors.As(err, &de) || de.Code != domain.ErrorCodeNotFound {
		t.Fatalf("expected NOT_FOUND, got %v", err)
	}
}
