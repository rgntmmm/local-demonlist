package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"local-demonlist/internal/middleware"
)

const sessionDuration = 7 * 24 * time.Hour

func createSession(c *gin.Context, pool *pgxpool.Pool, playerID int64) error {
	var random [32]byte
	rand.Read(random[:])
	token := base64.RawURLEncoding.EncodeToString(random[:])
	hash := sha256.Sum256([]byte(token))
	_, err := pool.Exec(c.Request.Context(),
		"INSERT INTO player_sessions (token_hash, player_id, expires_at) VALUES ($1, $2, $3)",
		hash[:], playerID, time.Now().Add(sessionDuration))
	if err != nil {
		return err
	}
	middleware.WriteSessionCookie(c, token, int(sessionDuration.Seconds()))
	return nil
}

func endSession(c *gin.Context, pool *pgxpool.Pool) error {
	if hash := middleware.SessionHash(c); hash != nil {
		if _, err := pool.Exec(c.Request.Context(),
			"DELETE FROM player_sessions WHERE token_hash = $1", hash); err != nil {
			return err
		}
	}
	middleware.WriteSessionCookie(c, "", -1)
	return nil
}

func CurrentSessionHandler(c *gin.Context) {
	middleware.NoStore(c)
	session, ok := c.Get("session")
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Sign in to continue."})
		return
	}
	c.JSON(http.StatusOK, session)
}

// EndSessionHandler завершает текущую сессию, включая повторный запрос выхода.
func EndSessionHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.NoStore(c)
		if err := endSession(c, pool); err != nil {
			log.Printf("end session: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not end the session."})
			return
		}
		c.Status(http.StatusNoContent)
	}
}
