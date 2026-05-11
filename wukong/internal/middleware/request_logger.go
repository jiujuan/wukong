package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger prints each request to the terminal.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		rawQuery := c.Request.URL.RawQuery
		requestID, _ := c.Get("RequestID")

		c.Next()

		latency := time.Since(start)
		if rawQuery != "" {
			path += "?" + rawQuery
		}

		log.Printf(
			"[HTTP] %s %s status=%d latency=%s ip=%s request_id=%v",
			c.Request.Method,
			path,
			c.Writer.Status(),
			latency,
			c.ClientIP(),
			requestID,
		)
	}
}
