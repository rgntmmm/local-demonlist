package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetInt64("accountID") == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Sign in to continue."})
			return
		}
		if c.GetString("accountRole") != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Administrator access is required."})
			return
		}
		c.Next()
	}
}

func GuestOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if id := c.GetInt64("accountID"); id != 0 {
			c.Redirect(http.StatusSeeOther, "/players/"+strconv.FormatInt(id, 10))
			c.Abort()
			return
		}
		c.Next()
	}
}
