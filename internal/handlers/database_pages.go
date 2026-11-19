package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"local-demonlist/internal/middleware"
	"local-demonlist/internal/queries"
)

type pageLevel struct {
	GameID     int64
	ID         int
	Name       string
	Author     string
	Difficulty string
	Position   int
	VideoURL   string
	Verifier   string
	Icon       string
	Points     int
}

type pagePlayer struct {
	ID          int64
	Name        string
	Rank        *int64
	Points      int64
	Completions int64
}

type pageRecord struct {
	PlayerID   int64
	PlayerName string
	LevelID    int
	LevelName  string
	Position   int
	Points     int
	VideoURL   string
}

func pageError(c *gin.Context, err error) {
	log.Printf("load page %s: %v", c.FullPath(), err)
	c.String(http.StatusInternalServerError, "Could not load page data")
}

func RegisterDatabasePages(r *gin.Engine, pool *pgxpool.Pool) {
	r.Use(middleware.CrossOriginProtection(), middleware.SessionMiddleware(pool))
	r.Use(func(c *gin.Context) { c.Set("pagePool", pool); c.Next() })
	r.GET("/archive", archivePageHandler(pool))
	registerAuthPages(r, pool)
	RegisterSettings(r, pool)
	RegisterLevelManagement(r, pool)
	RegisterCompletionRequests(r, pool)
	RegisterCompletionSubmission(r, pool)
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/demonlist")
	})
	r.GET("/demonlist", func(c *gin.Context) {
		query := strings.TrimSpace(c.Query("query"))
		rows, err := pool.Query(c.Request.Context(), queries.Pages("levels"), nil, query)
		if err != nil {
			pageError(c, err)
			return
		}
		levels, err := pgx.CollectRows(rows, pgx.RowToStructByPos[pageLevel])
		if err != nil {
			pageError(c, err)
			return
		}
		renderPage(c, http.StatusOK, "demonlist.html", gin.H{"Levels": levels, "Query": query})
	})
	r.GET("/leaderboard", func(c *gin.Context) {
		rows, err := pool.Query(c.Request.Context(), queries.Pages("players"), nil)
		if err != nil {
			pageError(c, err)
			return
		}
		players, err := pgx.CollectRows(rows, pgx.RowToStructByPos[pagePlayer])
		if err != nil {
			pageError(c, err)
			return
		}
		renderPage(c, http.StatusOK, "leaderboard.html", gin.H{"Players": players})
	})
	r.GET("/levels/:id", func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 32)
		if err != nil || id <= 0 {
			c.String(http.StatusNotFound, "Level not found")
			return
		}
		rows, err := pool.Query(c.Request.Context(), queries.Pages("levels"), id, "")
		if err != nil {
			pageError(c, err)
			return
		}
		level, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[pageLevel])
		if errors.Is(err, pgx.ErrNoRows) {
			c.String(http.StatusNotFound, "Level not found")
			return
		}
		if err != nil {
			pageError(c, err)
			return
		}
		rows, err = pool.Query(c.Request.Context(), queries.Pages("records"), nil, id)
		if err != nil {
			pageError(c, err)
			return
		}
		records, err := pgx.CollectRows(rows, pgx.RowToStructByPos[pageRecord])
		if err != nil {
			pageError(c, err)
			return
		}
		renderPage(c, http.StatusOK, "level.html", gin.H{"Level": level, "Records": records})
	})
	r.GET("/players/:id", func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			c.String(http.StatusNotFound, "Player not found")
			return
		}
		rows, err := pool.Query(c.Request.Context(), queries.Pages("players"), id)
		if err != nil {
			pageError(c, err)
			return
		}
		player, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByPos[pagePlayer])
		if errors.Is(err, pgx.ErrNoRows) {
			c.String(http.StatusNotFound, "Player not found")
			return
		}
		if err != nil {
			pageError(c, err)
			return
		}
		rows, err = pool.Query(c.Request.Context(), queries.Pages("records"), id, nil)
		if err != nil {
			pageError(c, err)
			return
		}
		records, err := pgx.CollectRows(rows, pgx.RowToStructByPos[pageRecord])
		if err != nil {
			pageError(c, err)
			return
		}
		renderPage(c, http.StatusOK, "player.html", gin.H{"Player": player, "Records": records})
	})
}
