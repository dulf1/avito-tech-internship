package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"prservice/internal/app/dto"
)

func ZapRecovery(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("panic recovered", zap.Any("panic", rec))
				c.AbortWithStatusJSON(http.StatusInternalServerError, dto.ErrorResponse{
					Error: dto.Error{
						Code:    "INTERNAL_ERROR",
						Message: "internal server error",
					},
				})
			}
		}()

		c.Next()
	}
}
