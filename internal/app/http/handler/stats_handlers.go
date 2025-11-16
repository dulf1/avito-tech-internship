package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"prservice/internal/app/dto"
)

func (h *Handler) StatsAssignments(c *gin.Context) {
	scope := strings.ToLower(c.DefaultQuery("scope", "all"))
	teamNameParam := c.Query("team_name")

	var teamName *string
	if teamNameParam != "" {
		teamName = &teamNameParam
	}

	type (
		UserStatDTO = dto.UserAssignmentStat
		PRAStatDTO  = dto.PRAssignmentStat
	)

	resp := dto.StatsResponse{}
	ctx := c.Request.Context()

	switch scope {
	case "all", "":
		userStats, err := h.StatsSvc.GetUserStats(ctx, teamName)
		if err != nil {
			h.writeError(c, err)
			return
		}

		resp.PerUser = make([]UserStatDTO, 0, len(userStats))
		for _, s := range userStats {
			resp.PerUser = append(resp.PerUser, UserStatDTO{
				UserID:         s.UserID,
				AssignedTotal:  s.AssignedTotal,
				AssignedOpen:   s.AssignedOpen,
				AssignedMerged: s.AssignedMerged,
			})
		}

		prStats, err := h.StatsSvc.GetPRStats(ctx)
		if err != nil {
			h.writeError(c, err)
			return
		}

		resp.PerPR = make([]PRAStatDTO, 0, len(prStats))
		for _, s := range prStats {
			resp.PerPR = append(resp.PerPR, PRAStatDTO{
				PullRequestID: s.PullRequestID,
				ReviewerCount: s.ReviewerCount,
			})
		}

	case "users":
		userStats, err := h.StatsSvc.GetUserStats(ctx, teamName)
		if err != nil {
			h.writeError(c, err)
			return
		}

		resp.PerUser = make([]UserStatDTO, 0, len(userStats))
		for _, s := range userStats {
			resp.PerUser = append(resp.PerUser, UserStatDTO{
				UserID:         s.UserID,
				AssignedTotal:  s.AssignedTotal,
				AssignedOpen:   s.AssignedOpen,
				AssignedMerged: s.AssignedMerged,
			})
		}

	case "prs":
		prStats, err := h.StatsSvc.GetPRStats(ctx)
		if err != nil {
			h.writeError(c, err)
			return
		}

		resp.PerPR = make([]PRAStatDTO, 0, len(prStats))
		for _, s := range prStats {
			resp.PerPR = append(resp.PerPR, PRAStatDTO{
				PullRequestID: s.PullRequestID,
				ReviewerCount: s.ReviewerCount,
			})
		}

	default:
		h.badRequest(c, "invalid scope, must be one of: all, users, prs")
		return
	}

	c.JSON(http.StatusOK, resp)
}
