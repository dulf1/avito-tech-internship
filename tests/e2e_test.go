package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"prservice/internal/app/dto"
)

var baseURL string

func init() {
	baseURL = os.Getenv("E2E_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
}

func postJSON(t *testing.T, path string, body any, wantStatus int, out any) {
	t.Helper()

	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+path, &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do POST %s: %v", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != wantStatus {
		var errBody map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		t.Fatalf("unexpected status %d (want %d), body=%v", resp.StatusCode, wantStatus, errBody)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
}

func getJSON(t *testing.T, path string, wantStatus int, out any) {
	t.Helper()

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(baseURL + path)
	if err != nil {
		t.Fatalf("do GET %s: %v", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != wantStatus {
		var errBody map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		t.Fatalf("unexpected status %d (want %d), body=%v", resp.StatusCode, wantStatus, errBody)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
}

func TestE2E_FullFlow(t *testing.T) {
	var healthResp map[string]any
	getJSON(t, "/health", http.StatusOK, &healthResp)

	teamName := "e2e-backend"

	teamBody := dto.Team{
		TeamName: teamName,
		Members: []dto.TeamMember{
			{UserID: "u1", Username: "Alice", IsActive: true},
			{UserID: "u2", Username: "Bob", IsActive: true},
			{UserID: "u3", Username: "Eve", IsActive: true},
		},
	}

	var teamResp struct {
		Team dto.Team `json:"team"`
	}

	respBuf := new(bytes.Buffer)
	reqBody, _ := json.Marshal(teamBody)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/team/add", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do POST /team/add: %v", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusCreated:
		if err := json.NewDecoder(resp.Body).Decode(&teamResp); err != nil {
			t.Fatalf("decode team response: %v", err)
		}
	case http.StatusBadRequest:
		_ = json.NewDecoder(resp.Body).Decode(respBuf)
	default:
		var errBody map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		t.Fatalf("unexpected status on /team/add: %d body=%v", resp.StatusCode, errBody)
	}

	prID := fmt.Sprintf("e2e-pr-%d", time.Now().UnixNano())

	createBody := map[string]string{
		"pull_request_id":   prID,
		"pull_request_name": "E2E test PR",
		"author_id":         "u1",
	}

	var prResp struct {
		PR dto.PullRequest `json:"pr"`
	}
	postJSON(t, "/pullRequest/create", createBody, http.StatusCreated, &prResp)

	if prResp.PR.PullRequestID != prID {
		t.Fatalf("unexpected pr id %q, want %q", prResp.PR.PullRequestID, prID)
	}
	if prResp.PR.Status != "OPEN" {
		t.Fatalf("expected status OPEN, got %s", prResp.PR.Status)
	}
	if len(prResp.PR.AssignedReviewers) == 0 {
		t.Fatalf("expected at least 1 reviewer")
	}
	if len(prResp.PR.AssignedReviewers) > 2 {
		t.Fatalf("expected <= 2 reviewers, got %d", len(prResp.PR.AssignedReviewers))
	}
	for _, r := range prResp.PR.AssignedReviewers {
		if r == "u1" {
			t.Fatalf("author must not be reviewer")
		}
	}

	reviewerID := prResp.PR.AssignedReviewers[0]

	mergeBody := map[string]string{
		"pull_request_id": prID,
	}

	var m1, m2 struct {
		PR dto.PullRequest `json:"pr"`
	}
	postJSON(t, "/pullRequest/merge", mergeBody, http.StatusOK, &m1)
	postJSON(t, "/pullRequest/merge", mergeBody, http.StatusOK, &m2)

	if m1.PR.Status != "MERGED" || m2.PR.Status != "MERGED" {
		t.Fatalf("expected MERGED status in both calls, got %s and %s", m1.PR.Status, m2.PR.Status)
	}
	if m1.PR.MergedAt == nil || m2.PR.MergedAt == nil {
		t.Fatalf("MergedAt must be non-nil after merge")
	}

	var reviewsResp struct {
		UserID       string                 `json:"user_id"`
		PullRequests []dto.PullRequestShort `json:"pull_requests"`
	}
	getJSON(t, "/users/getReview?user_id="+reviewerID, http.StatusOK, &reviewsResp)

	if reviewsResp.UserID != reviewerID {
		t.Fatalf("unexpected user_id in getReview: %s", reviewsResp.UserID)
	}

	found := false
	for _, prShort := range reviewsResp.PullRequests {
		if prShort.PullRequestID == prID {
			found = true
			if prShort.Status != "MERGED" {
				t.Fatalf("expected PR status MERGED in getReview, got %s", prShort.Status)
			}
		}
	}
	if !found {
		t.Fatalf("expected PR %s in /users/getReview for %s", prID, reviewerID)
	}

	var statsResp dto.StatsResponse
	getJSON(t, "/stats/assignments?scope=all", http.StatusOK, &statsResp)

	if len(statsResp.PerUser) == 0 && len(statsResp.PerPR) == 0 {
		t.Fatalf("expected some stats data, got empty response")
	}
}
