package stats_test

import (
	"context"
	"testing"

	"prservice/internal/domain/stats"
)

type repoFake struct {
	users []stats.UserAssignmentStat
	prs   []stats.PRAssignmentStat
}

func (r *repoFake) GetUserAssignmentStats(ctx context.Context, teamName *string) ([]stats.UserAssignmentStat, error) {
	return append([]stats.UserAssignmentStat(nil), r.users...), nil
}
func (r *repoFake) GetPRAssignmentStats(ctx context.Context) ([]stats.PRAssignmentStat, error) {
	return append([]stats.PRAssignmentStat(nil), r.prs...), nil
}

func TestStatsService_PassThrough(t *testing.T) {
	r := &repoFake{
		users: []stats.UserAssignmentStat{
			{UserID: "u1", AssignedTotal: 3, AssignedOpen: 1, AssignedMerged: 2},
		},
		prs: []stats.PRAssignmentStat{
			{PullRequestID: "pr-1", ReviewerCount: 2},
		},
	}
	svc := stats.NewService(r)

	us, err := svc.GetUserStats(context.Background(), nil)
	if err != nil || len(us) != 1 || us[0].UserID != "u1" {
		t.Fatalf("unexpected user stats: %v %v", us, err)
	}

	ps, err := svc.GetPRStats(context.Background())
	if err != nil || len(ps) != 1 || ps[0].PullRequestID != "pr-1" {
		t.Fatalf("unexpected pr stats: %v %v", ps, err)
	}
}
