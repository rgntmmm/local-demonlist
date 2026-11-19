package main

import (
	"context"
	"errors"
	"log"
	"os"

	"github.com/joho/godotenv"
	"local-demonlist/internal/database"

	"github.com/gin-gonic/gin"

	"local-demonlist/internal/handlers"
)

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatal("Could not read .env")
	}
	pool, err := database.Open(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	r := gin.Default()
	r.Static("/static", "./static")
	r.LoadHTMLGlob("templates/*.html")
	handlers.RegisterDatabasePages(r, pool)
	r.POST("/api/register", handlers.RegisterHandler(pool))

	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
