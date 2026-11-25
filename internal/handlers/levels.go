package handlers

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"local-demonlist/internal/middleware"
	"local-demonlist/internal/models"
	"local-demonlist/internal/queries"
)

func ListLevelsHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := pool.Query(c.Request.Context(), queries.Pages("levels"), nil, "")
		if err != nil {
			pageError(c, err)
			return
		}
		levels, err := pgx.CollectRows(rows, pgx.RowToStructByPos[pageLevel])
		if err != nil {
			pageError(c, err)
			return
		}
		c.JSON(http.StatusOK, levels)
	}
}

func RegisterLevelManagement(r *gin.Engine, pool *pgxpool.Pool) {
	admin := r.Group("/levels", middleware.RequireAdmin())
	admin.GET("/new", levelFormHandler(pool))
	admin.GET("/:id/edit", levelFormHandler(pool))
	admin.POST("", CreateLevelHandler(pool))
	admin.POST("/:id/edit", saveLevelHandler(pool, true))
	admin.POST("/:id/archive", archiveLevelHandler(pool))
	admin.POST("/:id/restore", restoreLevelHandler(pool))
	admin.POST("/:id/delete", deleteLevelHandler(pool))
}

func levelID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 32)
	if err != nil || id <= 0 {
		c.String(http.StatusNotFound, "Level not found")
		return 0, false
	}
	return id, true
}

func levelFormHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var form models.CreateLevelRequest
		var id int64
		if c.Param("id") == "" {
			if err := pool.QueryRow(c.Request.Context(),
				"SELECT COUNT(*) + 1 FROM levels WHERE archived_at IS NULL").Scan(&form.Position); err != nil {
				pageError(c, err)
				return
			}
		} else {
			var ok bool
			id, ok = levelID(c)
			if !ok {
				return
			}
			err := pool.QueryRow(c.Request.Context(), `
				SELECT game_id, name, author, difficulty, position, COALESCE(video_url, ''), COALESCE(verifier, ''), COALESCE(level_icon, '')
				FROM levels WHERE id = $1 AND archived_at IS NULL`, id).
				Scan(&form.GameID, &form.Name, &form.Author, &form.Difficulty, &form.Position, &form.VideoURL, &form.Verifier, &form.Icon)
			if errors.Is(err, pgx.ErrNoRows) {
				c.String(http.StatusNotFound, "Level not found")
				return
			}
			if err != nil {
				pageError(c, err)
				return
			}
		}
		renderPage(c, http.StatusOK, "add_level.html", gin.H{"Form": form, "EditID": id})
	}
}

func CreateLevelHandler(pool *pgxpool.Pool) gin.HandlerFunc {
	return saveLevelHandler(pool, false)
}

func saveLevelHandler(pool *pgxpool.Pool, editing bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Keep the exported handler protected when registered on another route.
		if c.GetInt64("accountID") == 0 || c.GetString("accountRole") != "admin" {
			middleware.RequireAdmin()(c)
			return
		}
		var id int64
		if editing {
			var ok bool
			id, ok = levelID(c)
			if !ok {
				return
			}
		}
		var form models.CreateLevelRequest
		jsonRequest := c.ContentType() == "application/json"
		fail := func(status int, message string) {
			if jsonRequest {
				c.JSON(status, gin.H{"error": message})
				return
			}
			renderPage(c, status, "add_level.html", gin.H{"Form": form, "EditID": id, "Error": message})
		}
		middleware.LimitRequestBody(c, 8192)
		if err := c.ShouldBind(&form); err != nil {
			fail(http.StatusBadRequest, "Enter a positive numeric game ID and check the level name, creator, difficulty, and placement.")
			return
		}
		form.Name = strings.TrimSpace(form.Name)
		form.Author = strings.TrimSpace(form.Author)
		form.VideoURL = strings.TrimSpace(form.VideoURL)
		if form.Name == "" || form.Author == "" || strings.ContainsRune(form.Name+form.Author, 0) {
			fail(http.StatusBadRequest, "Enter the level name and creator.")
			return
		}
		form.Verifier = strings.TrimSpace(form.Verifier)
		form.Icon = strings.TrimSpace(form.Icon)
		if strings.ContainsRune(form.Verifier, 0) {
			fail(http.StatusBadRequest, "Check the verifier name.")
			return
		}
		for _, link := range []string{form.VideoURL, form.Icon} {
			if link == "" {
				continue
			}
			u, err := url.Parse(link)
			if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
				fail(http.StatusBadRequest, "Video and icon URLs must start with http:// or https://.")
				return
			}
		}
		ctx := c.Request.Context()
		tx, err := pool.Begin(ctx)
		if err != nil {
			pageError(c, err)
			return
		}
		defer tx.Rollback(ctx)
		// Serializes all list changes before reading the current positions.
		if _, err = tx.Exec(ctx, "LOCK TABLE levels IN SHARE ROW EXCLUSIVE MODE"); err != nil {
			pageError(c, err)
			return
		}
		var count int
		if err = tx.QueryRow(ctx, "SELECT COUNT(*) FROM levels WHERE archived_at IS NULL").Scan(&count); err != nil {
			pageError(c, err)
			return
		}
		var oldPosition int
		if editing {
			err = tx.QueryRow(ctx, "SELECT position FROM levels WHERE id = $1 AND archived_at IS NULL", id).Scan(&oldPosition)
			if errors.Is(err, pgx.ErrNoRows) {
				c.String(http.StatusNotFound, "Level not found")
				return
			}
			if err != nil {
				pageError(c, err)
				return
			}
		} else {
			count++
		}
		if form.Position > count {
			fail(http.StatusBadRequest, "Highest allowed placement number: "+strconv.Itoa(count))
			return
		}
		if editing {
			_, err = tx.Exec(ctx, `
				UPDATE levels SET position = CASE WHEN $2::integer < $3::integer THEN position + 1 ELSE position - 1 END
				WHERE id <> $1 AND archived_at IS NULL AND
				(($2::integer < $3::integer AND position >= $2::integer AND position < $3::integer)
				 OR ($2::integer > $3::integer AND position > $3::integer AND position <= $2::integer))`, id, form.Position, oldPosition)
		} else {
			_, err = tx.Exec(ctx, queries.Levels("shift_positions"), form.Position)
		}
		if err != nil {
			pageError(c, err)
			return
		}
		if editing {
			_, err = tx.Exec(ctx, `
				UPDATE levels SET name=$2, author=$3, difficulty=$4, position=$5, video_url=NULLIF($6, ''), verifier=NULLIF($7, ''), level_icon=NULLIF($8, ''), game_id=$9
				WHERE id=$1`, id, form.Name, form.Author, form.Difficulty, form.Position, form.VideoURL, form.Verifier, form.Icon, form.GameID)
		} else {
			err = tx.QueryRow(ctx, queries.Levels("insert"), form.Name, form.Author, form.Difficulty, form.Position, form.VideoURL, form.Verifier, form.Icon, form.GameID).Scan(&id)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "22023" {
			fail(http.StatusBadRequest, "Verifier nickname is too long or matches multiple accounts.")
			return
		}
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "levels_game_id_unique" {
			fail(http.StatusConflict, "A level with this game ID already exists.")
			return
		}
		if err != nil {
			pageError(c, err)
			return
		}
		var points int
		if err = tx.QueryRow(ctx, "SELECT COALESCE((SELECT points FROM position_points WHERE position=$1), 0)", form.Position).Scan(&points); err != nil {
			pageError(c, err)
			return
		}
		if err = tx.Commit(ctx); err != nil {
			pageError(c, err)
			return
		}
		if jsonRequest {
			status := http.StatusCreated
			if editing {
				status = http.StatusOK
			}
			c.JSON(status, models.Level{GameID: form.GameID, ID: int(id), Name: form.Name, Author: form.Author,
				Difficulty: form.Difficulty, Position: form.Position, VideoURL: form.VideoURL, Verifier: form.Verifier, Icon: form.Icon, Points: points})
			return
		}
		c.Redirect(http.StatusSeeOther, "/levels/"+strconv.FormatInt(id, 10))
	}
}

func archiveLevelHandler(pool *pgxpool.Pool) gin.HandlerFunc {
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
		if _, err = tx.Exec(ctx, "LOCK TABLE levels IN SHARE ROW EXCLUSIVE MODE"); err != nil {
			pageError(c, err)
			return
		}
		var position *int
		err = tx.QueryRow(ctx, "SELECT position FROM levels WHERE id=$1", id).Scan(&position)
		if errors.Is(err, pgx.ErrNoRows) {
			c.String(http.StatusNotFound, "Level not found")
			return
		}
		if err != nil {
			pageError(c, err)
			return
		}
		if position != nil {
			if _, err = tx.Exec(ctx, "UPDATE levels SET position=NULL, archived_at=now() WHERE id=$1", id); err != nil {
				pageError(c, err)
				return
			}
			if _, err = tx.Exec(ctx, "UPDATE levels SET position=position-1 WHERE position>$1", *position); err != nil {
				pageError(c, err)
				return
			}
		}
		if err = tx.Commit(ctx); err != nil {
			pageError(c, err)
			return
		}
		c.Redirect(http.StatusSeeOther, "/levels/"+strconv.FormatInt(id, 10))
	}
}

// deleteLevelHandler removes the level and its completions in one transaction.
func deleteLevelHandler(pool *pgxpool.Pool) gin.HandlerFunc {
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
		if _, err = tx.Exec(ctx, "LOCK TABLE levels IN SHARE ROW EXCLUSIVE MODE"); err != nil {
			pageError(c, err)
			return
		}
		var position *int
		err = tx.QueryRow(ctx, "SELECT position FROM levels WHERE id=$1 FOR UPDATE", id).Scan(&position)
		if errors.Is(err, pgx.ErrNoRows) {
			c.String(http.StatusNotFound, "Level not found")
			return
		}
		if err != nil {
			pageError(c, err)
			return
		}
		// The row lock prevents new foreign-key references until commit.
		if _, err = tx.Exec(ctx, "DELETE FROM player_completions WHERE level_id=$1", id); err != nil {
			pageError(c, err)
			return
		}
		if _, err = tx.Exec(ctx, "DELETE FROM levels WHERE id=$1", id); err != nil {
			pageError(c, err)
			return
		}
		if position != nil {
			if _, err = tx.Exec(ctx, "UPDATE levels SET position=position-1 WHERE position>$1", *position); err != nil {
				pageError(c, err)
				return
			}
		}
		if err = tx.Commit(ctx); err != nil {
			pageError(c, err)
			return
		}
		c.Redirect(http.StatusSeeOther, "/demonlist")
	}
}
