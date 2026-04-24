package domain

import "github.com/gin-gonic/gin"

type ErrorPayload struct {
	Error     ErrorDetails `json:"error"`
	RequestID string       `json:"request_id,omitempty"`
}

type ErrorDetails struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteError(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorPayload{
		Error: ErrorDetails{
			Code:    code,
			Message: message,
		},
		RequestID: c.GetHeader("X-Request-ID"),
	})
}
