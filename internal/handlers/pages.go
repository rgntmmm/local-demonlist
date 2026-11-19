package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"local-demonlist/internal/models"
	"local-demonlist/internal/points"
)

type previewPlayer struct {
	ID          int
	Name        string
	Rank        int
	Points      int
	Completions int
}

type previewRecord struct {
	PlayerID   int
	PlayerName string
	LevelID    int
	LevelName  string
	Position   int
	Points     int
	VideoURL   string
}

// RegisterPreviewPages renders templates with demonstration data until database
// access and player models are implemented. It does not register write routes.
func RegisterPreviewPages(r *gin.Engine) {
	levels := []models.Level{
		{ID: 1, Name: "Sample Level A", Author: "ExampleCreator", Difficulty: "Extreme", Position: 1, Verifier: "ExamplePlayer", Points: points.PositionPoints[1]},
		{ID: 2, Name: "Sample Level B", Author: "AnotherCreator", Difficulty: "Insane", Position: 2, Points: points.PositionPoints[2]},
		{ID: 3, Name: "Sample Level C", Author: "ExampleCreator", Difficulty: "Hard", Position: 3, Points: points.PositionPoints[3]},
	}
	record := previewRecord{
		PlayerID: 1, PlayerName: "ExamplePlayer", LevelID: levels[0].ID,
		LevelName: levels[0].Name, Position: levels[0].Position, Points: levels[0].Points,
	}
	player := previewPlayer{
		ID: record.PlayerID, Name: record.PlayerName, Rank: 1,
		Points: record.Points, Completions: 1,
	}

	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/demonlist")
	})
	r.GET("/demonlist", func(c *gin.Context) {
		query := strings.TrimSpace(c.Query("query"))
		filtered := make([]models.Level, 0, len(levels))
		for _, level := range levels {
			if strings.Contains(strings.ToLower(level.Name), strings.ToLower(query)) ||
				strings.Contains(strings.ToLower(level.Author), strings.ToLower(query)) {
				filtered = append(filtered, level)
			}
		}
		c.HTML(http.StatusOK, "demonlist.html", gin.H{
			"Levels": filtered, "Query": query, "Preview": true,
		})
	})
	r.GET("/leaderboard", func(c *gin.Context) {
		c.HTML(http.StatusOK, "leaderboard.html", gin.H{
			"Players": []previewPlayer{player}, "Preview": true,
		})
	})
	r.GET("/levels/new", func(c *gin.Context) {
		c.HTML(http.StatusOK, "add_level.html", gin.H{
			"Form":  models.CreateLevelRequest{Position: len(levels) + 1},
			"Error": "", "CSRFToken": "", "Preview": true,
		})
	})
	r.GET("/levels/:id", func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err == nil {
			for _, level := range levels {
				if level.ID != id {
					continue
				}
				records := make([]previewRecord, 0)
				if level.ID == record.LevelID {
					records = append(records, record)
				}
				c.HTML(http.StatusOK, "level.html", gin.H{
					"Level": level, "Records": records, "Preview": true,
				})
				return
			}
		}
		c.String(http.StatusNotFound, "Level not found")
	})
	r.GET("/players/:id", func(c *gin.Context) {
		if c.Param("id") != "1" {
			c.String(http.StatusNotFound, "Player not found")
			return
		}
		c.HTML(http.StatusOK, "player.html", gin.H{
			"Player":  player,
			"Records": []previewRecord{record}, "Preview": true,
		})
	})
}
