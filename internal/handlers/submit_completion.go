package handlers

import (
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"local-demonlist/internal/middleware"
)

type completionSubmission struct {
	Level       string `form:"level" binding:"required,max=255"`
	Attempts    int    `form:"attempts" binding:"required,min=1,max=2147483647"`
	VideoURL    string `form:"video_url" binding:"max=2048"`
	ContactType string `form:"contact_type" binding:"required,oneof=email discord"`
	Contact     string `form:"contact" binding:"required,max=254"`
}

func RegisterCompletionSubmission(r *gin.Engine, pool *pgxpool.Pool) {
	signedIn := r.Group("/completion-requests", middleware.RequireSession())
	signedIn.GET("/new", func(c *gin.Context) {
		renderPage(c, http.StatusOK, "submit_completion.html", gin.H{
			"Submission": completionSubmission{}, "SubmissionSent": c.Query("sent") == "1",
		})
	})
	signedIn.POST("", SubmitCompletionHandler(pool))
}

func SubmitCompletionHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var form completionSubmission
		fail := func(status int, message string) {
			renderPage(c, status, "submit_completion.html", gin.H{
				"Submission": form, "SubmissionError": message,
			})
		}
		middleware.LimitRequestBody(c, 8192)
		if err := c.ShouldBind(&form); err != nil {
			fail(http.StatusBadRequest, "Enter a level, a positive attempt count, and your contact details.")
			return
		}
		form.Level = strings.TrimSpace(form.Level)
		form.Contact = strings.TrimSpace(form.Contact)
		form.VideoURL = strings.TrimSpace(form.VideoURL)
		if form.Level == "" || form.Contact == "" || !utf8.ValidString(form.Contact) ||
			strings.ContainsRune(form.Level+form.Contact, 0) {
			fail(http.StatusBadRequest, "Check the level and contact details.")
			return
		}
		if form.ContactType == "email" {
			address, err := mail.ParseAddress(form.Contact)
			if err != nil || address.Address != form.Contact {
				fail(http.StatusBadRequest, "Enter a valid email address.")
				return
			}
		}
		if form.VideoURL == "" && c.GetString("accountRole") != "admin" {
			fail(http.StatusBadRequest, "A completion video is required.")
			return
		}
		if form.VideoURL != "" {
			u, err := url.Parse(form.VideoURL)
			if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
				fail(http.StatusBadRequest, "Enter an http:// or https:// video URL.")
				return
			}
		}
		rows, err := pool.Query(c.Request.Context(),
			"SELECT id FROM levels WHERE game_id::text=$1 OR lower(name)=lower($1)", form.Level)
		if err != nil {
			pageError(c, err)
			return
		}
		ids, err := pgx.CollectRows(rows, pgx.RowTo[int])
		if err != nil {
			pageError(c, err)
			return
		}
		if len(ids) == 0 {
			fail(http.StatusBadRequest, "Level not found. Enter its exact name or Geometry Dash ID.")
			return
		}
		if len(ids) > 1 {
			fail(http.StatusBadRequest, "Several levels match. Enter the Geometry Dash ID.")
			return
		}
		result, err := pool.Exec(c.Request.Context(), `
            INSERT INTO player_completions(player_id,level_id,video_url,attempts,contact_type,contact)
            VALUES ($1,$2,NULLIF($3,''),$4,$5,$6)
            ON CONFLICT (player_id,level_id) DO NOTHING`,
			c.GetInt64("accountID"), ids[0], form.VideoURL, form.Attempts, form.ContactType, form.Contact)
		if err != nil {
			pageError(c, err)
			return
		}
		if result.RowsAffected() == 0 {
			fail(http.StatusConflict, "You already have a submission or completion for this level.")
			return
		}
		c.Redirect(http.StatusSeeOther, "/completion-requests/new?sent=1")
	}
}
