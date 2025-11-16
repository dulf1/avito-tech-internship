package pr

import (
	"context"
	"net/http"

	"prservice/internal/domain"
	"prservice/internal/domain/user"
)

type Service interface {
	Create(ctx context.Context, id, name, authorID string) (PullRequest, error)
	Merge(ctx context.Context, id string) (PullRequest, error)
	ReassignReviewer(ctx context.Context, prID, oldUserID string) (PullRequest, string, error)
	GetUserReviews(ctx context.Context, userID string) ([]PullRequestShort, error)
}

type service struct {
	uow    domain.UnitOfWork
	prs    Repository
	users  user.Repository
	events domain.EventBus
	rnd    domain.RandomSource
}

func NewService(
	uow domain.UnitOfWork,
	prs Repository,
	users user.Repository,
	events domain.EventBus,
	rnd domain.RandomSource,
) Service {
	return &service{
		uow:    uow,
		prs:    prs,
		users:  users,
		events: events,
		rnd:    rnd,
	}
}

func (s *service) Create(ctx context.Context, id, name, authorID string) (PullRequest, error) {
	var res PullRequest

	err := s.uow.WithinTx(ctx, func(ctx context.Context) error {
		author, err := s.users.GetByID(ctx, authorID)
		if err != nil {
			return err
		}
		if author.TeamName == "" {
			return &domain.DomainError{
				Code:       domain.ErrorCodeNotFound,
				Message:    "author has no team",
				HTTPStatus: http.StatusNotFound,
			}
		}

		candidates, err := s.users.GetActiveTeamMembersExcept(ctx, author.TeamName, author.ID)
		if err != nil {
			return err
		}

		ids := make([]string, 0, len(candidates))
		for _, u := range candidates {
			ids = append(ids, u.ID)
		}

		selected := randomSubset(s.rnd, ids, 2)

		pr := PullRequest{
			ID:                id,
			Name:              name,
			AuthorID:          author.ID,
			Status:            StatusOpen,
			AssignedReviewers: selected,
		}

		created, err := s.prs.CreateWithReviewers(ctx, pr)
		if err != nil {
			return err
		}
		res = created

		if s.events != nil {
			s.events.Publish(ctx, domain.Event{
				Type: "pr.created",
				Payload: map[string]any{
					"pr_id":     res.ID,
					"author_id": res.AuthorID,
				},
			})
		}

		return nil
	})

	return res, err
}

func (s *service) Merge(ctx context.Context, id string) (PullRequest, error) {
	var res PullRequest

	err := s.uow.WithinTx(ctx, func(ctx context.Context) error {
		current, err := s.prs.LockByID(ctx, id)
		if err != nil {
			return err
		}

		if current.Status == StatusMerged {
			revs, err := s.prs.GetReviewers(ctx, id)
			if err != nil {
				return err
			}
			current.AssignedReviewers = revs
			res = current
			return nil
		}

		updated, err := s.prs.UpdateStatusMerged(ctx, id)
		if err != nil {
			return err
		}
		revs, err := s.prs.GetReviewers(ctx, id)
		if err != nil {
			return err
		}
		updated.AssignedReviewers = revs
		res = updated

		if s.events != nil {
			s.events.Publish(ctx, domain.Event{
				Type:    "pr.merged",
				Payload: map[string]any{"pr_id": id},
			})
		}

		return nil
	})

	return res, err
}

func (s *service) ReassignReviewer(ctx context.Context, prID, oldUserID string) (PullRequest, string, error) {
	var res PullRequest
	var replacedBy string

	err := s.uow.WithinTx(ctx, func(ctx context.Context) error {
		current, err := s.prs.LockByID(ctx, prID)
		if err != nil {
			return err
		}
		if current.Status == StatusMerged {
			return &domain.DomainError{
				Code:       domain.ErrorCodePRMerged,
				Message:    "cannot reassign on merged PR",
				HTTPStatus: http.StatusConflict,
			}
		}

		oldUser, err := s.users.GetByID(ctx, oldUserID)
		if err != nil {
			return err
		}
		if oldUser.TeamName == "" {
			return &domain.DomainError{
				Code:       domain.ErrorCodeNotFound,
				Message:    "user has no team",
				HTTPStatus: http.StatusNotFound,
			}
		}

		assigned, err := s.prs.UserIsReviewer(ctx, prID, oldUserID)
		if err != nil {
			return err
		}
		if !assigned {
			return &domain.DomainError{
				Code:       domain.ErrorCodeNotAssigned,
				Message:    "reviewer is not assigned to this PR",
				HTTPStatus: http.StatusConflict,
			}
		}

		currentReviewers, err := s.prs.GetReviewers(ctx, prID)
		if err != nil {
			return err
		}

		candidatesUsers, err := s.users.GetActiveTeamMembersExcept(ctx, oldUser.TeamName, oldUser.ID)
		if err != nil {
			return err
		}

		filteredIDs := make([]string, 0, len(candidatesUsers))
		for _, u := range candidatesUsers {
			if u.ID == current.AuthorID {
				continue
			}
			if contains(currentReviewers, u.ID) {
				continue
			}
			filteredIDs = append(filteredIDs, u.ID)
		}
		if len(filteredIDs) == 0 {
			return &domain.DomainError{
				Code:       domain.ErrorCodeNoCandidate,
				Message:    "no active replacement candidate in team",
				HTTPStatus: http.StatusConflict,
			}
		}

		selected := randomSubset(s.rnd, filteredIDs, 1)
		replacedBy = selected[0]

		newReviewers := make([]string, 0, len(currentReviewers))
		for _, rID := range currentReviewers {
			if rID == oldUserID {
				newReviewers = append(newReviewers, replacedBy)
			} else {
				newReviewers = append(newReviewers, rID)
			}
		}

		if err := s.prs.SetReviewers(ctx, prID, newReviewers); err != nil {
			return err
		}

		current.AssignedReviewers = newReviewers
		res = current

		if s.events != nil {
			s.events.Publish(ctx, domain.Event{
				Type: "pr.reassign",
				Payload: map[string]any{
					"pr_id":       prID,
					"old_user":    oldUserID,
					"replaced_by": replacedBy,
				},
			})
		}

		return nil
	})

	return res, replacedBy, err
}

func (s *service) GetUserReviews(ctx context.Context, userID string) ([]PullRequestShort, error) {
	return s.prs.GetUserPRs(ctx, userID)
}

func randomSubset(r domain.RandomSource, items []string, max int) []string {
	n := len(items)
	if n == 0 || max <= 0 {
		return nil
	}
	out := make([]string, n)
	copy(out, items)
	r.Shuffle(n, func(i, j int) { out[i], out[j] = out[j], out[i] })
	if n > max {
		out = out[:max]
	}
	return out
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
