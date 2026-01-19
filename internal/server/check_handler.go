package server

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github/martinmaurice/rlim/pkg/rate_limiter"
	"log/slog"
	"net/http"
)

type (
	checkHandlerServicer interface {
		GetRate(key string) (rate_limiter.Rate, error)
	}
	checkRequestDTO struct {
		Key  string `json:"key"`
		Tier string `json:"tier"`
		Cost int    `json:"cost"`
	}
	checkResponseDTO struct {
		Allowed    bool `json:"allowed"`
		Limit      int  `json:"limit"`
		Remaining  int  `json:"remaining"`
		ResetAt    int  `json:"reset_at"`
		RetryAfter int  `json:"retry_after"`
	}
)

func CheckHandler(s checkHandlerServicer) func(ctx *gin.Context) {
	return func(ctx *gin.Context) {
		var reqDTO checkRequestDTO
		err := ctx.ShouldBind(&reqDTO)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		res, err := s.GetRate(reqDTO.Key)
		if err != nil {
			slog.Error(fmt.Sprintf("%v", err))
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, err)
			return
		}

		respDTO := checkResponseDTO{
			Allowed:    res.RemainingTokens > reqDTO.Cost,
			Limit:      100,
			Remaining:  res.RemainingTokens,
			ResetAt:    res.LastRefill,
			RetryAfter: 42,
		}

		ctx.JSON(http.StatusOK, respDTO)
	}
}
