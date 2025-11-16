package team_test

import (
	"context"
	"errors"
	"testing"

	"prservice/internal/domain"
	"prservice/internal/domain/team"
	"prservice/internal/domain/user"
)

type uowStub struct{}

func (uowStub) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type eventBusFake struct{ events []domain.Event }

func (e *eventBusFake) Publish(ctx context.Context, ev domain.Event) { e.events = append(e.events, ev) }

type userRepoFake struct {
	byID map[string]user.User
}

func newUserRepoFake() *userRepoFake { return &userRepoFake{byID: map[string]user.User{}} }

func (r *userRepoFake) UpsertInTeam(ctx context.Context, teamName string, members []user.User) error {
	for _, u := range members {
		u.TeamName = teamName
		r.byID[u.ID] = u
	}
	return nil
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
func (r *userRepoFake) GetByID(ctx context.Context, userID string) (user.User, error) {
	u, ok := r.byID[userID]
	if !ok {
		return user.User{}, &domain.DomainError{Code: domain.ErrorCodeNotFound, Message: "user not found", HTTPStatus: 404}
	}
	return u, nil
}
func (r *userRepoFake) GetActiveTeamMembersExcept(ctx context.Context, teamName, excludeUserID string) ([]user.User, error) {
	var res []user.User
	for _, u := range r.byID {
		if u.TeamName == teamName && u.IsActive && u.ID != excludeUserID {
			res = append(res, u)
		}
	}
	return res, nil
}

type teamRepoFake struct {
	created map[string]bool
	users   *userRepoFake
}

func newTeamRepoFake(u *userRepoFake) *teamRepoFake {
	return &teamRepoFake{created: map[string]bool{}, users: u}
}

func (r *teamRepoFake) Exists(ctx context.Context, name string) (bool, error) {
	return r.created[name], nil
}
func (r *teamRepoFake) Create(ctx context.Context, name string) error {
	if r.created[name] {
		return errors.New("exists")
	}
	r.created[name] = true
	return nil
}
func (r *teamRepoFake) GetWithMembers(ctx context.Context, name string) (team.Team, error) {
	if !r.created[name] {
		return team.Team{}, &domain.DomainError{Code: domain.ErrorCodeNotFound, Message: "team not found", HTTPStatus: 404}
	}
	var members []team.Member
	for _, u := range r.users.byID {
		if u.TeamName == name {
			members = append(members, team.Member{ID: u.ID, Username: u.Username, IsActive: u.IsActive})
		}
	}
	return team.Team{Name: name, Members: members}, nil
}

func TestAddTeam_Success(t *testing.T) {
	uow := uowStub{}
	users := newUserRepoFake()
	teams := newTeamRepoFake(users)
	events := &eventBusFake{}

	svc := team.NewService(uow, teams, users, events)

	tm := team.Team{
		Name: "backend",
		Members: []team.Member{
			{ID: "u1", Username: "Alice", IsActive: true},
			{ID: "u2", Username: "Bob", IsActive: false},
		},
	}

	got, err := svc.AddTeam(context.Background(), tm)
	if err != nil {
		t.Fatalf("AddTeam: %v", err)
	}
	if got.Name != "backend" || len(got.Members) != 2 {
		t.Fatalf("unexpected result: %+v", got)
	}
	if !teams.created["backend"] {
		t.Fatalf("team was not created in repo")
	}
	if len(events.events) != 1 || events.events[0].Type != "team.created" {
		t.Fatalf("expected team.created event, got %+v", events.events)
	}
}

func TestAddTeam_AlreadyExists(t *testing.T) {
	uow := uowStub{}
	users := newUserRepoFake()
	teams := newTeamRepoFake(users)
	events := &eventBusFake{}

	teams.created["backend"] = true

	svc := team.NewService(uow, teams, users, events)
	_, err := svc.AddTeam(context.Background(), team.Team{Name: "backend"})
	var de *domain.DomainError
	if err == nil || !errors.As(err, &de) || de.Code != domain.ErrorCodeTeamExists {
		t.Fatalf("expected TEAM_EXISTS, got %v", err)
	}
}

func TestGetTeam(t *testing.T) {
	uow := uowStub{}
	users := newUserRepoFake()
	teams := newTeamRepoFake(users)
	events := &eventBusFake{}

	teams.created["backend"] = true
	_ = users.UpsertInTeam(context.Background(), "backend", []user.User{
		{ID: "u1", Username: "Alice", TeamName: "backend", IsActive: true},
		{ID: "u2", Username: "Bob", TeamName: "backend", IsActive: false},
	})

	svc := team.NewService(uow, teams, users, events)
	got, err := svc.GetTeam(context.Background(), "backend")
	if err != nil {
		t.Fatalf("GetTeam: %v", err)
	}
	if got.Name != "backend" || len(got.Members) != 2 {
		t.Fatalf("unexpected team: %+v", got)
	}
}
