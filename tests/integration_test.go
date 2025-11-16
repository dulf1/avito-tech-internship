package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"prservice/internal/app/dto"
	httpapi "prservice/internal/app/http"
	"prservice/internal/app/http/handler"
	"prservice/internal/domain/pr"
	"prservice/internal/domain/stats"
	"prservice/internal/domain/team"
	userdomain "prservice/internal/domain/user"
	"prservice/internal/infrastructure/async"
	"prservice/internal/infrastructure/db/pg"
	"prservice/internal/infrastructure/logging"
)

type testRandSource struct{}

func (testRandSource) Shuffle(n int, swap func(i, j int)) {
}

var migrateOnce sync.Once

func ensureMigrations(t *testing.T, db *sql.DB) {
	t.Helper()

	migrateOnce.Do(func() {
		if err := goose.SetDialect("postgres"); err != nil {
			t.Fatalf("goose.SetDialect: %v", err)
		}

		dir := "migrations"
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			alt := filepath.Join("..", "migrations")
			if _, err2 := os.Stat(alt); err2 == nil {
				dir = alt
			} else {
				t.Fatalf("migrations directory not found: tried %q (%v) and %q (%v)", dir, err, alt, err2)
			}
		}

		if err := goose.Up(db, dir); err != nil {
			t.Fatalf("goose.Up: %v", err)
		}
	})
}

func resetDB(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := db.ExecContext(ctx, `
		TRUNCATE TABLE pull_request_reviewers, pull_requests, users, teams
		RESTART IDENTITY CASCADE;
	`); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		host := getenvDefault("POSTGRES_HOST", "localhost")
		user := getenvDefault("POSTGRES_USER", "pruser")
		pass := getenvDefault("POSTGRES_PASSWORD", "prpass")
		port := getenvDefault("POSTGRES_PORT", "5432")
		dbname := getenvDefault("POSTGRES_DB", "prdb")

		dsn = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
			user, pass, host, port, dbname)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		t.Fatalf("ping db: %v", err)
	}

	ensureMigrations(t, db)
	resetDB(t, db)

	return db
}

func setupTestServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()

	db := getTestDB(t)

	log, err := logging.NewLogger()
	if err != nil {
		_ = db.Close()
		t.Fatalf("create logger: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	eventBus := async.NewAsyncEventBus(ctx, 4, log)
	uow := pg.NewTxManager(db)

	teamRepo := pg.NewTeamRepository(db)
	userRepo := pg.NewUserRepository(db)
	prRepo := pg.NewPRRepository(db)
	statsRepo := pg.NewStatsRepository(db)

	teamSvc := team.NewService(uow, teamRepo, userRepo, eventBus)
	userSvc := userdomain.NewService(uow, userRepo, eventBus)
	prSvc := pr.NewService(uow, prRepo, userRepo, eventBus, testRandSource{})
	statsSvc := stats.NewService(statsRepo)

	h := handler.New(teamSvc, userSvc, prSvc, statsSvc, log)
	router := httpapi.NewRouter(h, log)

	ts := httptest.NewServer(router)

	cleanup := func() {
		ts.Close()
		eventBus.Close()
		cancel()
		_ = log.Sync()
		_ = db.Close()
	}

	return ts, cleanup
}

func doPost(t *testing.T, client *http.Client, url string, body any, wantStatus int, out any) {
	t.Helper()

	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}

	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != wantStatus {
		var errBody map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		t.Fatalf("unexpected status %d, want %d, body=%v", resp.StatusCode, wantStatus, errBody)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
}

func doGet(t *testing.T, client *http.Client, url string, wantStatus int, out any) {
	t.Helper()

	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("do GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != wantStatus {
		var errBody map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		t.Fatalf("unexpected status %d, want %d, body=%v", resp.StatusCode, wantStatus, errBody)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
}

func TestIntegration_CreateTeamAndPRFlow(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	client := &http.Client{Timeout: 2 * time.Second}

	teamBody := dto.Team{
		TeamName: "backend",
		Members: []dto.TeamMember{
			{UserID: "u1", Username: "Alice", IsActive: true},
			{UserID: "u2", Username: "Bob", IsActive: true},
			{UserID: "u3", Username: "Eve", IsActive: true},
		},
	}

	var teamResp struct {
		Team dto.Team `json:"team"`
	}

	doPost(t, client, ts.URL+"/team/add", teamBody, http.StatusCreated, &teamResp)

	if teamResp.Team.TeamName != "backend" {
		t.Fatalf("unexpected team_name %q", teamResp.Team.TeamName)
	}

	if len(teamResp.Team.Members) != 3 {
		t.Fatalf("expected 3 members, got %d", len(teamResp.Team.Members))
	}

	createBody := map[string]string{
		"pull_request_id":   "pr-1001",
		"pull_request_name": "Add search",
		"author_id":         "u1",
	}

	var prResp struct {
		PR dto.PullRequest `json:"pr"`
	}

	doPost(t, client, ts.URL+"/pullRequest/create", createBody, http.StatusCreated, &prResp)

	if prResp.PR.PullRequestID != "pr-1001" {
		t.Fatalf("unexpected pr id %q", prResp.PR.PullRequestID)
	}

	if prResp.PR.AuthorID != "u1" {
		t.Fatalf("unexpected author_id %q", prResp.PR.AuthorID)
	}

	if prResp.PR.Status != "OPEN" {
		t.Fatalf("expected status OPEN, got %s", prResp.PR.Status)
	}

	if len(prResp.PR.AssignedReviewers) == 0 {
		t.Fatalf("expected at least 1 assigned reviewer")
	}

	if len(prResp.PR.AssignedReviewers) > 2 {
		t.Fatalf("expected <= 2 reviewers, got %d", len(prResp.PR.AssignedReviewers))
	}

	for _, r := range prResp.PR.AssignedReviewers {
		if r == "u1" {
			t.Fatalf("author must not be assigned as reviewer")
		}
	}
}

func TestIntegration_MergeIsIdempotent(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	client := &http.Client{Timeout: 2 * time.Second}

	teamBody := dto.Team{
		TeamName: "backend",
		Members: []dto.TeamMember{
			{UserID: "u1", Username: "Alice", IsActive: true},
			{UserID: "u2", Username: "Bob", IsActive: true},
			{UserID: "u3", Username: "Eve", IsActive: true},
		},
	}

	doPost(t, client, ts.URL+"/team/add", teamBody, http.StatusCreated, nil)

	createBody := map[string]string{
		"pull_request_id":   "pr-2001",
		"pull_request_name": "Feature X",
		"author_id":         "u1",
	}

	var prResp struct {
		PR dto.PullRequest `json:"pr"`
	}

	doPost(t, client, ts.URL+"/pullRequest/create", createBody, http.StatusCreated, &prResp)

	mergeBody := map[string]string{
		"pull_request_id": prResp.PR.PullRequestID,
	}

	var m1, m2 struct {
		PR dto.PullRequest `json:"pr"`
	}

	doPost(t, client, ts.URL+"/pullRequest/merge", mergeBody, http.StatusOK, &m1)
	doPost(t, client, ts.URL+"/pullRequest/merge", mergeBody, http.StatusOK, &m2)

	if m1.PR.Status != "MERGED" || m2.PR.Status != "MERGED" {
		t.Fatalf("expected both responses to be MERGED, got %s and %s", m1.PR.Status, m2.PR.Status)
	}

	if m1.PR.MergedAt == nil || m2.PR.MergedAt == nil {
		t.Fatalf("MergedAt must not be nil after merge")
	}

	if !m1.PR.MergedAt.Equal(*m2.PR.MergedAt) {
		t.Fatalf("MergedAt differs between idempotent calls: %v vs %v", m1.PR.MergedAt, m2.PR.MergedAt)
	}
}

func TestIntegration_ReassignReviewer(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	client := &http.Client{Timeout: 2 * time.Second}

	teamBody := dto.Team{
		TeamName: "backend",
		Members: []dto.TeamMember{
			{UserID: "u1", Username: "Alice", IsActive: true},
			{UserID: "u2", Username: "Bob", IsActive: true},
			{UserID: "u3", Username: "Eve", IsActive: true},
			{UserID: "u4", Username: "John", IsActive: true},
			{UserID: "u5", Username: "Kate", IsActive: true},
		},
	}

	doPost(t, client, ts.URL+"/team/add", teamBody, http.StatusCreated, nil)

	createBody := map[string]string{
		"pull_request_id":   "pr-3001",
		"pull_request_name": "Refactor",
		"author_id":         "u1",
	}

	var prResp struct {
		PR dto.PullRequest `json:"pr"`
	}

	doPost(t, client, ts.URL+"/pullRequest/create", createBody, http.StatusCreated, &prResp)

	if len(prResp.PR.AssignedReviewers) == 0 {
		t.Fatalf("need at least one reviewer for reassign test")
	}

	oldReviewer := prResp.PR.AssignedReviewers[0]

	reassignBody := map[string]string{
		"pull_request_id": prResp.PR.PullRequestID,
		"old_user_id":     oldReviewer,
	}

	var reasResp struct {
		PR         dto.PullRequest `json:"pr"`
		ReplacedBy string          `json:"replaced_by"`
	}

	doPost(t, client, ts.URL+"/pullRequest/reassign", reassignBody, http.StatusOK, &reasResp)

	if reasResp.PR.Status != "OPEN" {
		t.Fatalf("expected OPEN status after reassign, got %s", reasResp.PR.Status)
	}

	if reasResp.ReplacedBy == "" {
		t.Fatalf("replaced_by must be set")
	}

	if reasResp.ReplacedBy == oldReviewer {
		t.Fatalf("replaced_by must differ from old reviewer")
	}

	foundNew := false
	foundOld := false
	for _, r := range reasResp.PR.AssignedReviewers {
		if r == reasResp.ReplacedBy {
			foundNew = true
		}
		if r == oldReviewer {
			foundOld = true
		}
	}

	if !foundNew {
		t.Fatalf("new reviewer %s not found in assigned_reviewers", reasResp.ReplacedBy)
	}

	if foundOld {
		t.Fatalf("old reviewer %s must be removed from assigned_reviewers", oldReviewer)
	}
}

func TestIntegration_UserGetReview(t *testing.T) {
	ts, cleanup := setupTestServer(t)
	defer cleanup()

	client := &http.Client{Timeout: 2 * time.Second}

	teamBody := dto.Team{
		TeamName: "backend",
		Members: []dto.TeamMember{
			{UserID: "u1", Username: "Alice", IsActive: true},
			{UserID: "u2", Username: "Bob", IsActive: true},
			{UserID: "u3", Username: "Eve", IsActive: true},
		},
	}

	doPost(t, client, ts.URL+"/team/add", teamBody, http.StatusCreated, nil)

	createBody := map[string]string{
		"pull_request_id":   "pr-4001",
		"pull_request_name": "Bugfix",
		"author_id":         "u1",
	}

	var prResp struct {
		PR dto.PullRequest `json:"pr"`
	}

	doPost(t, client, ts.URL+"/pullRequest/create", createBody, http.StatusCreated, &prResp)

	if len(prResp.PR.AssignedReviewers) == 0 {
		t.Fatalf("expected at least 1 reviewer")
	}

	reviewerID := prResp.PR.AssignedReviewers[0]

	var getResp struct {
		UserID       string                 `json:"user_id"`
		PullRequests []dto.PullRequestShort `json:"pull_requests"`
	}

	doGet(t, client, ts.URL+"/users/getReview?user_id="+reviewerID, http.StatusOK, &getResp)

	if getResp.UserID != reviewerID {
		t.Fatalf("unexpected user_id in response: %s", getResp.UserID)
	}

	found := false
	for _, prShort := range getResp.PullRequests {
		if prShort.PullRequestID == prResp.PR.PullRequestID {
			found = true
			if prShort.Status != "OPEN" {
				t.Fatalf("expected status OPEN, got %s", prShort.Status)
			}
		}
	}

	if !found {
		t.Fatalf("expected PR %s in user reviews", prResp.PR.PullRequestID)
	}
}
