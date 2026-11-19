package handlers

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"local-demonlist/internal/middleware"
	"local-demonlist/internal/queries"
)

var validLogin = regexp.MustCompile(`^[A-Za-z0-9_]{3,32}$`)

func renderPage(c *gin.Context, status int, name string, data gin.H) {
	if value, ok := c.Get("pagePool"); ok {
		var hasArchive bool
		if err := value.(*pgxpool.Pool).QueryRow(c.Request.Context(), "SELECT EXISTS(SELECT 1 FROM levels WHERE archived_at IS NOT NULL)").Scan(&hasArchive); err != nil {
			pageError(c, err)
			return
		}
		data["HasArchive"] = hasArchive
	}
	data["IsAdmin"] = c.GetString("accountRole") == "admin"
	data["AccountID"] = c.GetInt64("accountID")
	middleware.NoStore(c)
	c.HTML(status, name, data)
}

func registerAuthPages(r *gin.Engine, pool *pgxpool.Pool) {
	r.GET("/api/session", middleware.RequireSession(), CurrentSessionHandler)
	r.DELETE("/api/session", EndSessionHandler(pool))

	r.GET("/register", middleware.GuestOnly(), func(c *gin.Context) {
		renderPage(c, http.StatusOK, "register.html", gin.H{})
	})
	r.POST("/register", middleware.GuestOnly(), func(c *gin.Context) {
		middleware.LimitRequestBody(c, 4096)
		if err := c.Request.ParseForm(); err != nil {
			c.String(http.StatusBadRequest, "Could not read the form")
			return
		}
		login := strings.TrimSpace(c.PostForm("login"))
		name := strings.TrimSpace(c.PostForm("display_name"))
		password := c.PostForm("password")
		data := gin.H{"Login": login, "DisplayName": name}
		fail := func(status int, message string) {
			data["Error"] = message
			renderPage(c, status, "register.html", data)
		}

		if message := registrationError(login, name, password); message != "" {
			fail(http.StatusBadRequest, message)
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			pageError(c, err)
			return
		}
		_, err = pool.Exec(c.Request.Context(), queries.InsertPlayer,
			login, name, string(hash))
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "players_login_unique" {
			fail(http.StatusConflict, "This username is already taken.")
			return
		}
		if err != nil {
			pageError(c, err)
			return
		}
		c.Redirect(http.StatusSeeOther, "/login?registered=1")
	})
	r.GET("/login", middleware.GuestOnly(), func(c *gin.Context) {
		renderPage(c, http.StatusOK, "login.html", gin.H{"Registered": c.Query("registered") == "1", "PasswordChanged": c.Query("password_changed") == "1"})
	})
	r.POST("/login", middleware.GuestOnly(), func(c *gin.Context) {
		middleware.LimitRequestBody(c, 4096)
		if err := c.Request.ParseForm(); err != nil {
			c.String(http.StatusBadRequest, "Could not read the form")
			return
		}
		login := strings.TrimSpace(c.PostForm("login"))
		password := c.PostForm("password")
		invalidCredentials := func() {
			renderPage(c, http.StatusUnauthorized, "login.html", gin.H{
				"Login": login, "Error": "Incorrect username or password.",
			})
		}
		if !validLogin.MatchString(login) || password == "" || len(password) > 72 {
			invalidCredentials()
			return
		}
		var id int64
		var passwordHash string
		ctx := c.Request.Context()
		err := pool.QueryRow(ctx, `
			SELECT id, password_hash FROM players WHERE lower(login) = lower($1) AND is_active`, login).
			Scan(&id, &passwordHash)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			pageError(c, err)
			return
		}
		// Compare a dummy hash for missing accounts as well.
		if errors.Is(err, pgx.ErrNoRows) {
			passwordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
		}
		if !passwordMatches(passwordHash, password) || id == 0 {
			invalidCredentials()
			return
		}
		if err := createSession(c, pool, id); err != nil {
			pageError(c, err)
			return
		}
		c.Redirect(http.StatusSeeOther, "/players/"+strconv.FormatInt(id, 10))
	})
	r.POST("/logout", func(c *gin.Context) {
		if err := endSession(c, pool); err != nil {
			pageError(c, err)
			return
		}
		c.Redirect(http.StatusSeeOther, "/login")
	})
}
