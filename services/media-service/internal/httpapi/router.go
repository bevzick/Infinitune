package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const healthcheckTimeout = 2 * time.Second

type DatabaseCheck interface {
	Ping(context.Context) error
}

type StorageCheck interface {
	Check(context.Context) error
}

type Dependencies struct {
	Database DatabaseCheck
	Storage  StorageCheck
}

func NewRouter(dependencies Dependencies) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.GET("/health", health(dependencies))
	return router
}

func health(dependencies Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), healthcheckTimeout)
		defer cancel()
		if dependencies.Database.Ping(ctx) != nil || dependencies.Storage.Check(ctx) != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
