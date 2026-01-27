package server

import (
	"github.com/gin-gonic/gin"
	"github/martinmaurice/rlim/internal/server/middleware"
	"log/slog"
	"net/http"
	"time"
)

func healthHandler(ctx *gin.Context) {
	logger := slog.With("handler", "health")
	reqArrivalTime, exists := ctx.Get(middleware.ReqArrivalTimeContextValueKey)
	if exists {
		queueTime := time.Since(reqArrivalTime.(time.Time)).Microseconds()
		logger.Info("Queue Time (ms)", "queueTime", queueTime)
	}

	ctx.JSON(http.StatusOK, gin.H{"success": true})
}
