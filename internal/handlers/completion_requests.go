package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"local-demonlist/internal/middleware"
	"local-demonlist/internal/queries"
)

type completionRequest struct {
	GameID      int64
	Attempts    *int
	ContactType string
	Contact     string
	ID          int64
	PlayerID    int64
	PlayerName  string
	LevelID     int
	LevelName   string
	VideoURL    string
	SubmittedAt string
}

func RegisterCompletionRequests(r *gin.Engine, pool *pgxpool.Pool) {
	admin := r.Group("/admin/completion-requests", middleware.RequireAdmin())
	admin.GET("", CompletionRequestsHandler(pool))
	admin.POST("/:id/review", ReviewCompletionHandler(pool))
}

func renderCompletionRequests(c *gin.Context, pool *pgxpool.Pool, status int, message string) {
	rows, err := pool.Query(c.Request.Context(), queries.Pages("completion_requests"))
	if err != nil {
		pageError(c, err)
		return
	}
	requests, err := pgx.CollectRows(rows, pgx.RowToStructByPos[completionRequest])
	if err != nil {
		pageError(c, err)
		return
	}
	renderPage(c, status, "completion_requests.html", gin.H{"Requests": requests, "Error": message})
}

func CompletionRequestsHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		renderCompletionRequests(c, pool, http.StatusOK, "")
	}
}

func ReviewCompletionHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			renderCompletionRequests(c, pool, http.StatusNotFound, "Submission not found.")
			return
		}
		middleware.LimitRequestBody(c, 4096)
		if err := c.Request.ParseForm(); err != nil {
			renderCompletionRequests(c, pool, http.StatusBadRequest, "Could not read the review decision.")
			return
		}
		decision := c.PostForm("decision")
		if decision != "approved" && decision != "rejected" {
			renderCompletionRequests(c, pool, http.StatusBadRequest, "Choose whether to approve or reject the submission.")
			return
		}
		result, err := pool.Exec(c.Request.Context(), `
			UPDATE player_completions
			SET status=$2, reviewed_by=$3, reviewed_at=now()
			WHERE id=$1 AND status='pending' AND player_id <> $3`, id, decision, c.GetInt64("accountID"))
		if err != nil {
			pageError(c, err)
			return
		}
		if result.RowsAffected() == 0 {
			var exists, ownSubmission bool
			if err := pool.QueryRow(c.Request.Context(),
				"SELECT EXISTS(SELECT 1 FROM player_completions WHERE id=$1), EXISTS(SELECT 1 FROM player_completions WHERE id=$1 AND player_id=$2)", id, c.GetInt64("accountID")).Scan(&exists, &ownSubmission); err != nil {
				pageError(c, err)
				return
			}
			status, message := http.StatusNotFound, "Submission not found."
			if ownSubmission {
				status, message = http.StatusForbidden, "Another administrator must review your submission."
			} else if exists {
				status, message = http.StatusConflict, "This submission has already been reviewed. The queue has been refreshed."
			}
			renderCompletionRequests(c, pool, status, message)
			return
		}
		c.Redirect(http.StatusSeeOther, "/admin/completion-requests")
	}
}
