package handler

import (
	"prservice/internal/domain/pr"
	"prservice/internal/domain/stats"
	"prservice/internal/domain/team"
	"prservice/internal/domain/user"

	"go.uber.org/zap"
)

type Handler struct {
	TeamSvc  team.Service
	UserSvc  user.Service
	PRSvc    pr.Service
	StatsSvc stats.Service
	Log      *zap.Logger
}

func New(
	teamSvc team.Service,
	userSvc user.Service,
	prSvc pr.Service,
	statsSvc stats.Service,
	log *zap.Logger,
) *Handler {
	return &Handler{
		TeamSvc:  teamSvc,
		UserSvc:  userSvc,
		PRSvc:    prSvc,
		StatsSvc: statsSvc,
		Log:      log,
	}
}
