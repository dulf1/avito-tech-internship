package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"prservice/internal/app/dto"
	"prservice/internal/domain"
)

func (h *Handler) writeError(c *gin.Context, err error) {
	var de *domain.DomainError
	if errors.As(err, &de) {
		c.JSON(de.HTTPStatus, dto.ErrorResponse{
			Error: dto.Error{
				Code:    string(de.Code),
				Message: de.Message,
			},
		})
		return
	}

	h.Log.Error("internal error", zap.Error(err))
	c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
		Error: dto.Error{
			Code:    "INTERNAL_ERROR",
			Message: "internal server error",
		},
	})
}

func (h *Handler) badRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, dto.ErrorResponse{
		Error: dto.Error{
			Code:    "BAD_REQUEST",
			Message: msg,
		},
	})
}
