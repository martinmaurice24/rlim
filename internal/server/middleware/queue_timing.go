package middleware

import (
	"github.com/gin-gonic/gin"
	"time"
)

const ReqArrivalTimeContextValueKey = "reqArrivalTime"

func QueueTimeMiddleware(c *gin.Context) {
	c.Set(ReqArrivalTimeContextValueKey, time.Now())
	c.Next()
}
