package config

import "time"

// Config содержит настройки приложения.
type Config struct {
	ServerAddress   string
	PostgresDSN     string
	AppMode         string
	ShutdownTimeout time.Duration
}