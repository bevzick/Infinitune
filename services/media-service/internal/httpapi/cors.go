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
			}
		}
		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		c.Next()
	}
}
