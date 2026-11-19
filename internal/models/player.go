package models

import "time"

type Player struct {
	ID           int64
	Login        string
	DisplayName  string
	PasswordHash string `json:"-"`
	Role         string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type PlayerCompletion struct {
	Attempts           *int
	ContactType        *string
	Contact            *string
	ConfirmationSource string
	ID                 int64
	PlayerID           int64
	LevelID            int
	VideoURL           *string
	CompletedAt        *time.Time
	Status             string
	ReviewedBy         *int64
	ReviewedAt         *time.Time
	CreatedAt          time.Time
}

type PositionPoints struct {
	Position int
	Points   int
}

type PlayerLeaderboardEntry struct {
	PlayerID       int64
	DisplayName    string
	Role           string
	CompletedCount int64
	TotalPoints    int64
	Rank           *int64
}
