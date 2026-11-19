package handlers

import (
	"errors"
	"local-demonlist/internal/middleware"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"local-demonlist/internal/queries"
)

type registerRequest struct {
	Login       string `json:"login"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

type registeredPlayer struct {
	ID          int64     `json:"id"`
	Login       string    `json:"login"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

func registrationError(login, name, password string) string {
	switch {
	case !validLogin.MatchString(login):
		return "Use 3 to 32 ASCII letters, digits, or underscores for your username."
	case name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 64 || strings.ContainsRune(name, 0):
		return "Your display name must be 1 to 64 characters long."
	case !utf8.ValidString(password) || utf8.RuneCountInString(password) < 8 || len(password) > 72:
		return "Your password must be at least 8 characters long and no more than 72 bytes."
	}
	return ""
}

// RegisterHandler создаёт аккаунт игрока без автоматического входа.
func RegisterHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.NoStore(c)
		middleware.LimitRequestBody(c, 4096)
		var req registerRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid registration details."})
			return
		}
		req.Login = strings.TrimSpace(req.Login)
		req.DisplayName = strings.TrimSpace(req.DisplayName)
		if message := registrationError(req.Login, req.DisplayName, req.Password); message != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": message})
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create the account."})
			return
		}
		var player registeredPlayer
		err = pool.QueryRow(c.Request.Context(), queries.InsertPlayer, req.Login, req.DisplayName, string(hash)).Scan(
			&player.ID, &player.Login, &player.DisplayName, &player.Role, &player.CreatedAt,
		)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "players_login_unique" {
			c.JSON(http.StatusConflict, gin.H{"error": "This username is already taken."})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create the account."})
			return
		}
		c.JSON(http.StatusCreated, player)
	}
}
