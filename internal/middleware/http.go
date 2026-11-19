package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func CrossOriginProtection() gin.HandlerFunc {
	protection := http.NewCrossOriginProtection()
	return func(c *gin.Context) {
		if err := protection.Check(c.Request); err != nil {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}

// LimitRequestBody is applied before the handler parses form or JSON input.
func LimitRequestBody(c *gin.Context, limit int64) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
}

func NoStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
}
