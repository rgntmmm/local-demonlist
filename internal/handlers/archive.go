package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"local-demonlist/internal/queries"
)

func archivePageHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := pool.Query(c.Request.Context(), queries.Pages("archive"))
		if err != nil {
			pageError(c, err)
			return
		}
		levels, err := pgx.CollectRows(rows, pgx.RowToStructByPos[pageLevel])
		if err != nil {
			pageError(c, err)
			return
		}
		renderPage(c, http.StatusOK, "archive.html", gin.H{"Levels": levels})
	}
}

func restoreLevelHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := levelID(c)
		if !ok {
			return
		}
		ctx := c.Request.Context()
		tx, err := pool.Begin(ctx)
		if err != nil {
			pageError(c, err)
			return
		}
		defer tx.Rollback(ctx)
		// Use the same lock as creation, moving, deletion and archiving.
		if _, err = tx.Exec(ctx, "LOCK TABLE levels IN SHARE ROW EXCLUSIVE MODE"); err != nil {
			pageError(c, err)
			return
		}
		var archived bool
		err = tx.QueryRow(ctx, "SELECT archived_at IS NOT NULL FROM levels WHERE id=$1", id).Scan(&archived)
		if errors.Is(err, pgx.ErrNoRows) {
			c.String(http.StatusNotFound, "Level not found")
			return
		}
		if err != nil {
			pageError(c, err)
			return
		}
		// A repeated submission must not move an already restored level.
		if archived {
			if _, err = tx.Exec(ctx, queries.Levels("restore"), id); err != nil {
				pageError(c, err)
				return
			}
		}
		if err = tx.Commit(ctx); err != nil {
			pageError(c, err)
			return
		}
		c.Redirect(http.StatusSeeOther, "/archive")
	}
}
