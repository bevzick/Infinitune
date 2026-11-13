package httpapi

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
)

func cors(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Vary", "Origin")
			if slices.Contains(allowedOrigins, origin) {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Expose-Headers", requestIDHeader)
			}
		}
		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Headers", "Content-Type, X-Request-ID")
			c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		c.Next()
	}
}
