package models

type Level struct {
	GameID     int64
	ID         int
	Name       string
	Author     string
	Difficulty string
	Position   int
	VideoURL   string
	Created_at string
	Verifier   string
	Icon       string
	Points     int
}

type CreateLevelRequest struct {
	GameID     int64  `json:"game_id" form:"game_id" binding:"required,min=1,max=9007199254740991"`
	Verifier   string `json:"verifier" form:"verifier" binding:"max=64"`
	Icon       string `json:"icon" form:"icon"`
	Name       string `json:"name" form:"name"       binding:"required,min=1,max=255"`
	Author     string `json:"author" form:"author"     binding:"required,min=1,max=255"`
	Difficulty string `json:"difficulty" form:"difficulty" binding:"required,oneof=Easy Medium Hard Insane Extreme"`
	Position   int    `json:"position" form:"position"   binding:"required,min=1"`
	VideoURL   string `json:"video_url" form:"video_url"`
}
