package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"prservice/internal/app/dto"
)

func (h *Handler) PRCreate(c *gin.Context) {
	var body struct {
		PullRequestID   string `json:"pull_request_id"`
		PullRequestName string `json:"pull_request_name"`
		AuthorID        string `json:"author_id"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		h.badRequest(c, "invalid JSON")
		return
	}

	if body.PullRequestID == "" || body.PullRequestName == "" || body.AuthorID == "" {
		h.badRequest(c, "pull_request_id, pull_request_name, author_id are required")
		return
	}

	pr, err := h.PRSvc.Create(c.Request.Context(), body.PullRequestID, body.PullRequestName, body.AuthorID)
	if err != nil {
		h.writeError(c, err)
		return
	}

	resp := struct {
		PR dto.PullRequest `json:"pr"`
	}{
		PR: dto.PullRequest{
			PullRequestID:     pr.ID,
			PullRequestName:   pr.Name,
			AuthorID:          pr.AuthorID,
			Status:            string(pr.Status),
			AssignedReviewers: append([]string(nil), pr.AssignedReviewers...),
			CreatedAt:         pr.CreatedAt,
			MergedAt:          pr.MergedAt,
		},
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) PRMerge(c *gin.Context) {
	var body struct {
		PullRequestID string `json:"pull_request_id"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		h.badRequest(c, "invalid JSON")
		return
	}

	if body.PullRequestID == "" {
		h.badRequest(c, "pull_request_id is required")
		return
	}

	pr, err := h.PRSvc.Merge(c.Request.Context(), body.PullRequestID)
	if err != nil {
		h.writeError(c, err)
		return
	}

	resp := struct {
		PR dto.PullRequest `json:"pr"`
	}{
		PR: dto.PullRequest{
			PullRequestID:     pr.ID,
			PullRequestName:   pr.Name,
			AuthorID:          pr.AuthorID,
			Status:            string(pr.Status),
			AssignedReviewers: append([]string(nil), pr.AssignedReviewers...),
			CreatedAt:         pr.CreatedAt,
			MergedAt:          pr.MergedAt,
		},
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) PRReassign(c *gin.Context) {
	var body struct {
		PullRequestID string `json:"pull_request_id"`
		OldUserID     string `json:"old_user_id"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		h.badRequest(c, "invalid JSON")
		return
	}

	if body.PullRequestID == "" || body.OldUserID == "" {
		h.badRequest(c, "pull_request_id and old_user_id are required")
		return
	}

	pr, replacedBy, err := h.PRSvc.ReassignReviewer(c.Request.Context(), body.PullRequestID, body.OldUserID)
	if err != nil {
		h.writeError(c, err)
		return
	}

	resp := struct {
		PR         dto.PullRequest `json:"pr"`
		ReplacedBy string          `json:"replaced_by"`
	}{
		PR: dto.PullRequest{
			PullRequestID:     pr.ID,
			PullRequestName:   pr.Name,
			AuthorID:          pr.AuthorID,
			Status:            string(pr.Status),
			AssignedReviewers: append([]string(nil), pr.AssignedReviewers...),
			CreatedAt:         pr.CreatedAt,
			MergedAt:          pr.MergedAt,
		},
		ReplacedBy: replacedBy,
	}

	c.JSON(http.StatusOK, resp)
}
