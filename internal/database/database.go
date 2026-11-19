package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Open создаёт пул и проверяет доступность PostgreSQL.
// Вызывающий код должен закрыть пул после завершения всех запросов.
func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("database connection string is empty")
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		// Ошибка парсера может содержать строку подключения с паролем.
		return nil, fmt.Errorf("invalid database connection configuration")
	}
	cfg.MaxConns = 10
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
