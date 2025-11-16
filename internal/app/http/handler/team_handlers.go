package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"prservice/internal/app/dto"
	"prservice/internal/domain/team"
)

func (h *Handler) TeamAdd(c *gin.Context) {
	var body dto.Team
	if err := c.ShouldBindJSON(&body); err != nil {
		h.badRequest(c, "invalid JSON")
		return
	}
	if body.TeamName == "" {
		h.badRequest(c, "team_name is required")
		return
	}

	t := team.Team{
		Name:    body.TeamName,
		Members: make([]team.Member, 0, len(body.Members)),
	}
	for _, m := range body.Members {
		t.Members = append(t.Members, team.Member{
			ID:       m.UserID,
			Username: m.Username,
			IsActive: m.IsActive,
		})
	}

	res, err := h.TeamSvc.AddTeam(c.Request.Context(), t)
	if err != nil {
		h.writeError(c, err)
		return
	}

	resp := struct {
		Team dto.Team `json:"team"`
	}{
		Team: dto.Team{
			TeamName: res.Name,
			Members:  make([]dto.TeamMember, 0, len(res.Members)),
		},
	}

	for _, m := range res.Members {
		resp.Team.Members = append(resp.Team.Members, dto.TeamMember{
			UserID:   m.ID,
			Username: m.Username,
			IsActive: m.IsActive,
		})
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) TeamGet(c *gin.Context) {
	teamName := c.Query("team_name")
	if teamName == "" {
		h.badRequest(c, "team_name is required")
		return
	}

	res, err := h.TeamSvc.GetTeam(c.Request.Context(), teamName)
	if err != nil {
		h.writeError(c, err)
		return
	}

	resp := dto.Team{
		TeamName: res.Name,
		Members:  make([]dto.TeamMember, 0, len(res.Members)),
	}
	for _, m := range res.Members {
		resp.Members = append(resp.Members, dto.TeamMember{
			UserID:   m.ID,
			Username: m.Username,
			IsActive: m.IsActive,
		})
	}
	c.JSON(http.StatusOK, resp)
}
