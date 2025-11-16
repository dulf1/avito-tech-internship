package pr_test

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"prservice/internal/domain"
	"prservice/internal/domain/pr"
	"prservice/internal/domain/user"
)

type uowStub struct{}

func (uowStub) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type eventBusFake struct {
	events []domain.Event
}

func (e *eventBusFake) Publish(ctx context.Context, ev domain.Event) {
	e.events = append(e.events, ev)
}

type fixedRand struct{}

func (fixedRand) Shuffle(n int, swap func(i, j int)) {
}

type userRepoFake struct {
	byID        map[string]user.User
	teamMembers map[string][]user.User
}

func newUserRepoFake() *userRepoFake {
	return &userRepoFake{
		byID:        map[string]user.User{},
		teamMembers: map[string][]user.User{},
	}
}

func (r *userRepoFake) UpsertInTeam(ctx context.Context, teamName string, members []user.User) error {
	r.teamMembers[teamName] = append([]user.User{}, members...)
	for _, u := range members {
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
	for tn, list := range r.teamMembers {
		for i := range list {
			if list[i].ID == userID {
				list[i].IsActive = isActive
			}
		}
		r.teamMembers[tn] = list
	}
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
	for _, u := range r.teamMembers[teamName] {
		if !u.IsActive || u.ID == excludeUserID {
			continue
		}
		res = append(res, u)
	}
	return res, nil
}

type prRepoFake struct {
	prs       map[string]pr.PullRequest
	reviewers map[string][]string
}

func newPRRepoFake() *prRepoFake {
	return &prRepoFake{
		prs:       map[string]pr.PullRequest{},
		reviewers: map[string][]string{},
	}
}

func (r *prRepoFake) CreateWithReviewers(ctx context.Context, p pr.PullRequest) (pr.PullRequest, error) {
	if _, ok := r.prs[p.ID]; ok {
		return pr.PullRequest{}, &domain.DomainError{Code: domain.ErrorCodePRExists, Message: "PR id already exists", HTTPStatus: 409}
	}
	r.prs[p.ID] = p
	r.reviewers[p.ID] = append([]string{}, p.AssignedReviewers...)
	return p, nil
}
func (r *prRepoFake) LockByID(ctx context.Context, id string) (pr.PullRequest, error) {
	p, ok := r.prs[id]
	if !ok {
		return pr.PullRequest{}, &domain.DomainError{Code: domain.ErrorCodeNotFound, Message: "pull request not found", HTTPStatus: 404}
	}
	return p, nil
}
func (r *prRepoFake) UpdateStatusMerged(ctx context.Context, id string) (pr.PullRequest, error) {
	p, ok := r.prs[id]
	if !ok {
		return pr.PullRequest{}, &domain.DomainError{Code: domain.ErrorCodeNotFound, Message: "pull request not found", HTTPStatus: 404}
	}
	now := time.Now().UTC()
	p.Status = pr.StatusMerged
	p.MergedAt = &now
	r.prs[id] = p
	return p, nil
}
func (r *prRepoFake) GetReviewers(ctx context.Context, prID string) ([]string, error) {
	return append([]string{}, r.reviewers[prID]...), nil
}
func (r *prRepoFake) SetReviewers(ctx context.Context, prID string, reviewerIDs []string) error {
	if _, ok := r.prs[prID]; !ok {
		return &domain.DomainError{Code: domain.ErrorCodeNotFound, Message: "pull request not found", HTTPStatus: 404}
	}
	r.reviewers[prID] = append([]string{}, reviewerIDs...)
	p := r.prs[prID]
	p.AssignedReviewers = append([]string{}, reviewerIDs...)
	r.prs[prID] = p
	return nil
}
func (r *prRepoFake) UserIsReviewer(ctx context.Context, prID, userID string) (bool, error) {
	for _, id := range r.reviewers[prID] {
		if id == userID {
			return true, nil
		}
	}
	return false, nil
}
func (r *prRepoFake) GetUserPRs(ctx context.Context, userID string) ([]pr.PullRequestShort, error) {
	var res []pr.PullRequestShort
	for _, p := range r.prs {
		for _, rid := range r.reviewers[p.ID] {
			if rid == userID {
				res = append(res, pr.PullRequestShort{ID: p.ID, Name: p.Name, AuthorID: p.AuthorID, Status: p.Status})
				break
			}
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i].ID < res[j].ID })
	return res, nil
}

func TestService_Create_AssignsUpToTwoActiveFromAuthorTeam(t *testing.T) {
	uow := uowStub{}
	users := newUserRepoFake()
	prs := newPRRepoFake()
	events := &eventBusFake{}
	rnd := fixedRand{}

	users.UpsertInTeam(context.Background(), "backend", []user.User{
		{ID: "u1", Username: "Alice", TeamName: "backend", IsActive: true},
		{ID: "u2", Username: "Bob", TeamName: "backend", IsActive: true},
		{ID: "u3", Username: "Eve", TeamName: "backend", IsActive: true},
		{ID: "u4", Username: "Tom", TeamName: "backend", IsActive: false},
		{ID: "u5", Username: "Dan", TeamName: "backend", IsActive: true},
	})
	svc := pr.NewService(uow, prs, users, events, rnd)

	users.byID["u1"] = user.User{ID: "u1", Username: "Alice", TeamName: "backend", IsActive: true}

	p, err := svc.Create(context.Background(), "pr-1", "Add search", "u1")
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if p.Status != pr.StatusOpen {
		t.Fatalf("expected status OPEN, got %s", p.Status)
	}
	if len(p.AssignedReviewers) == 0 || len(p.AssignedReviewers) > 2 {
		t.Fatalf("expected 1..2 reviewers, got %v", p.AssignedReviewers)
	}
	for _, r := range p.AssignedReviewers {
		if r == "u1" {
			t.Fatalf("author must not be reviewer")
		}
		if r == "u4" {
			t.Fatalf("inactive must not be reviewer")
		}
	}
	if len(events.events) != 1 || events.events[0].Type != "pr.created" {
		t.Fatalf("expected pr.created event, got %+v", events.events)
	}
}

func TestService_Create_AuthorWithoutTeam_ReturnsNotFound(t *testing.T) {
	uow := uowStub{}
	users := newUserRepoFake()
	prs := newPRRepoFake()
	events := &eventBusFake{}
	rnd := fixedRand{}

	users.byID["u1"] = user.User{ID: "u1", Username: "Alice", TeamName: "", IsActive: true}

	svc := pr.NewService(uow, prs, users, events, rnd)
	_, err := svc.Create(context.Background(), "pr-err", "X", "u1")
	if err == nil {
		t.Fatal("expected error")
	}
	var de *domain.DomainError
	if !errors.As(err, &de) || de.Code != domain.ErrorCodeNotFound {
		t.Fatalf("expected NOT_FOUND, got %v", err)
	}
}

func TestService_Merge_NormalAndIdempotent(t *testing.T) {
	uow := uowStub{}
	users := newUserRepoFake()
	prs := newPRRepoFake()
	events := &eventBusFake{}
	rnd := fixedRand{}
	svc := pr.NewService(uow, prs, users, events, rnd)

	prs.prs["pr-2"] = pr.PullRequest{ID: "pr-2", Name: "X", AuthorID: "u1", Status: pr.StatusOpen}
	prs.reviewers["pr-2"] = []string{"u2", "u3"}

	p, err := svc.Merge(context.Background(), "pr-2")
	if err != nil {
		t.Fatalf("merge error: %v", err)
	}
	if p.Status != pr.StatusMerged || p.MergedAt == nil {
		t.Fatalf("expected MERGED with mergedAt, got %+v", p)
	}
	if len(events.events) != 1 || events.events[0].Type != "pr.merged" {
		t.Fatalf("expected one pr.merged event, got %+v", events.events)
	}

	events.events = nil
	p2, err := svc.Merge(context.Background(), "pr-2")
	if err != nil {
		t.Fatalf("idempotent merge error: %v", err)
	}
	if p2.Status != pr.StatusMerged {
		t.Fatalf("expected MERGED, got %s", p2.Status)
	}
	if len(events.events) != 0 {
		t.Fatalf("no event expected on idempotent merge")
	}
}

func TestService_Reassign_Success(t *testing.T) {
	uow := uowStub{}
	users := newUserRepoFake()
	prs := newPRRepoFake()
	events := &eventBusFake{}
	rnd := fixedRand{}
	svc := pr.NewService(uow, prs, users, events, rnd)

	users.UpsertInTeam(context.Background(), "backend", []user.User{
		{ID: "u1", Username: "Alice", TeamName: "backend", IsActive: true},
		{ID: "u2", Username: "Bob", TeamName: "backend", IsActive: true},
		{ID: "u3", Username: "Eve", TeamName: "backend", IsActive: true},
		{ID: "u4", Username: "Tom", TeamName: "backend", IsActive: true},
	})

	prs.prs["pr-3"] = pr.PullRequest{ID: "pr-3", Name: "X", AuthorID: "u1", Status: pr.StatusOpen}
	prs.reviewers["pr-3"] = []string{"u2", "u3"}

	users.byID["u2"] = user.User{ID: "u2", Username: "Bob", TeamName: "backend", IsActive: true}

	users.teamMembers["backend"] = []user.User{
		{ID: "u1", Username: "Alice", TeamName: "backend", IsActive: true},
		{ID: "u4", Username: "Tom", TeamName: "backend", IsActive: true},
		{ID: "u3", Username: "Eve", TeamName: "backend", IsActive: true},
		{ID: "u2", Username: "Bob", TeamName: "backend", IsActive: true},
	}

	p, replacedBy, err := svc.ReassignReviewer(context.Background(), "pr-3", "u2")
	if err != nil {
		t.Fatalf("reassign error: %v", err)
	}
	if replacedBy != "u4" {
		t.Fatalf("expected replacement u4, got %s", replacedBy)
	}
	if !contains(p.AssignedReviewers, "u4") || contains(p.AssignedReviewers, "u2") {
		t.Fatalf("reviewers not replaced correctly: %v", p.AssignedReviewers)
	}
	if len(events.events) == 0 || events.events[0].Type != "pr.reassign" {
		t.Fatalf("expected pr.reassign event, got %+v", events.events)
	}
}

func TestService_Reassign_Errors(t *testing.T) {
	uow := uowStub{}
	users := newUserRepoFake()
	prs := newPRRepoFake()
	events := &eventBusFake{}
	rnd := fixedRand{}
	svc := pr.NewService(uow, prs, users, events, rnd)

	prs.prs["pr-m"] = pr.PullRequest{ID: "pr-m", Name: "X", AuthorID: "u1", Status: pr.StatusMerged}
	_, _, err := svc.ReassignReviewer(context.Background(), "pr-m", "u2")
	if !isDomainErr(err, domain.ErrorCodePRMerged) {
		t.Fatalf("want PR_MERGED, got %v", err)
	}

	prs.prs["pr-x"] = pr.PullRequest{ID: "pr-x", Name: "X", AuthorID: "u1", Status: pr.StatusOpen}
	prs.reviewers["pr-x"] = []string{"u2"}
	users.byID["u2"] = user.User{ID: "u2", Username: "Bob", TeamName: "", IsActive: true}
	_, _, err = svc.ReassignReviewer(context.Background(), "pr-x", "u2")
	if !isDomainErr(err, domain.ErrorCodeNotFound) {
		t.Fatalf("want NOT_FOUND (no team), got %v", err)
	}

	users.byID["u2"] = user.User{ID: "u2", Username: "Bob", TeamName: "backend", IsActive: true}
	prs.reviewers["pr-x"] = []string{"u3"}
	_, _, err = svc.ReassignReviewer(context.Background(), "pr-x", "u2")
	if !isDomainErr(err, domain.ErrorCodeNotAssigned) {
		t.Fatalf("want NOT_ASSIGNED, got %v", err)
	}

	prs.reviewers["pr-x"] = []string{"u2"}
	users.teamMembers["backend"] = []user.User{
		{ID: "u1", Username: "Alice", TeamName: "backend", IsActive: true},
		{ID: "u2", Username: "Bob", TeamName: "backend", IsActive: true},
	}
	_, _, err = svc.ReassignReviewer(context.Background(), "pr-x", "u2")
	if !isDomainErr(err, domain.ErrorCodeNoCandidate) {
		t.Fatalf("want NO_CANDIDATE, got %v", err)
	}
}

func TestService_GetUserReviews(t *testing.T) {
	uow := uowStub{}
	users := newUserRepoFake()
	prs := newPRRepoFake()
	events := &eventBusFake{}
	rnd := fixedRand{}
	svc := pr.NewService(uow, prs, users, events, rnd)

	prs.prs["pr-a"] = pr.PullRequest{ID: "pr-a", Name: "A", AuthorID: "u1", Status: pr.StatusOpen}
	prs.prs["pr-b"] = pr.PullRequest{ID: "pr-b", Name: "B", AuthorID: "u2", Status: pr.StatusOpen}
	prs.reviewers["pr-a"] = []string{"u9"}
	prs.reviewers["pr-b"] = []string{"u8", "u9"}

	list, err := svc.GetUserReviews(context.Background(), "u9")
	if err != nil {
		t.Fatalf("GetUserReviews: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 PRs, got %d", len(list))
	}
}

func isDomainErr(err error, code domain.ErrorCode) bool {
	var de *domain.DomainError
	return errors.As(err, &de) && de.Code == code
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
