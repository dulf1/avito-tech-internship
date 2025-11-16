package httpapi

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"prservice/internal/app/http/handler"
	"prservice/internal/app/http/middleware"
)

func NewRouter(h *handler.Handler, log *zap.Logger) *gin.Engine {
	r := gin.New()

	r.Use(
		gin.Recovery(),
		middleware.ZapLogger(log),
		middleware.ZapRecovery(log),
	)

	r.GET("/health", h.Health)

	r.POST("/team/add", h.TeamAdd)
	r.GET("/team/get", h.TeamGet)

	r.POST("/users/setIsActive", h.UserSetIsActive)
	r.GET("/users/getReview", h.UserGetReview)

	r.POST("/pullRequest/create", h.PRCreate)
	r.POST("/pullRequest/merge", h.PRMerge)
	r.POST("/pullRequest/reassign", h.PRReassign)

	r.GET("/stats/assignments", h.StatsAssignments)

	return r
}
