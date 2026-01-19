package server

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github/martinmaurice/rlim/internal/server/middleware"
	"log/slog"
	"net/http"
	"time"
)

func GetStatusByIdHandler(c *gin.Context)    {}
func DeleteRateByIdHandler(c *gin.Context)   {}
func UpdateRateConfigHandler(c *gin.Context) {}
func HealthHandler(c *gin.Context) {
	reqArrivalTime, exists := c.Get(middleware.ReqArrivalTimeContextValueKey)
	if exists {
		queueTime := time.Since(reqArrivalTime.(time.Time)).Microseconds()
		slog.Info(fmt.Sprintf("Queue Time: %v Microseconds", queueTime))
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
func MetricHandler(c *gin.Context) {}
