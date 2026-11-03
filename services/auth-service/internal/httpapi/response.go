package httpapi

import "github.com/gin-gonic/gin"

type errorResponse struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, errorResponse{
		Error: apiError{
			Code:    code,
			Message: message,
		},
	})
}
