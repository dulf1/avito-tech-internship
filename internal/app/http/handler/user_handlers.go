package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"prservice/internal/app/dto"
)

func (h *Handler) UserSetIsActive(c *gin.Context) {
	var body struct {
		UserID   string `json:"user_id"`
		IsActive bool   `json:"is_active"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		h.badRequest(c, "invalid JSON")
		return
	}
	if body.UserID == "" {
		h.badRequest(c, "user_id is required")
		return
	}

	u, err := h.UserSvc.SetUserActive(c.Request.Context(), body.UserID, body.IsActive)
	if err != nil {
		h.writeError(c, err)
		return
	}

	resp := struct {
		User dto.User `json:"user"`
	}{
		User: dto.User{
			UserID:   u.ID,
			Username: u.Username,
			TeamName: u.TeamName,
			IsActive: u.IsActive,
		},
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) UserGetReview(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		h.badRequest(c, "user_id is required")
		return
	}

	list, err := h.PRSvc.GetUserReviews(c.Request.Context(), userID)
	if err != nil {
		h.writeError(c, err)
		return
	}

	resp := struct {
		UserID       string                 `json:"user_id"`
		PullRequests []dto.PullRequestShort `json:"pull_requests"`
	}{
		UserID:       userID,
		PullRequests: make([]dto.PullRequestShort, 0, len(list)),
	}

	for _, pr := range list {
		resp.PullRequests = append(resp.PullRequests, dto.PullRequestShort{
			PullRequestID:   pr.ID,
			PullRequestName: pr.Name,
			AuthorID:        pr.AuthorID,
			Status:          string(pr.Status),
		})
	}

	c.JSON(http.StatusOK, resp)
}
