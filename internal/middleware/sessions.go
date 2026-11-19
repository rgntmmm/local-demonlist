package middleware

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"net/http"
	"time"
)

const SessionCookie = "local_demonlist_session"

type currentSession struct {
	PlayerID    int64     `json:"player_id"`
	Login       string    `json:"login"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func SessionHash(c *gin.Context) []byte {
	token, err := c.Cookie(SessionCookie)
	if err != nil || len(token) != 43 {
		return nil
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(decoded) != 32 {
		return nil
	}
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}

func WriteSessionCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: SessionCookie, Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: c.Request.TLS != nil, SameSite: http.SameSiteLaxMode,
	})
}

// SessionMiddleware загружает действующую сессию и актуальные данные аккаунта.
func SessionMiddleware(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		NoStore(c)
		hash := SessionHash(c)
		if hash == nil {
			c.Next()
			return
		}
		var session currentSession
		err := pool.QueryRow(c.Request.Context(), `
			SELECT p.id, p.login, p.display_name, p.role, s.expires_at
			FROM player_sessions s JOIN players p ON p.id = s.player_id
			WHERE s.token_hash = $1 AND s.expires_at > now() AND p.is_active`, hash).
			Scan(&session.PlayerID, &session.Login, &session.DisplayName, &session.Role, &session.ExpiresAt)
		if errors.Is(err, pgx.ErrNoRows) {
			WriteSessionCookie(c, "", -1)
			c.Next()
			return
		}
		if err != nil {
			log.Printf("load session: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Could not verify the session."})
			return
		}
		c.Set("session", session)
		c.Set("accountID", session.PlayerID)
		c.Set("accountRole", session.Role)
		c.Next()
	}
}

func RequireSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetInt64("accountID") == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Sign in to continue."})
			return
		}
		c.Next()
	}
}
