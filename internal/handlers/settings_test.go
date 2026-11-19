package handlers

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func TestAvatarValidation(t *testing.T) {
	for _, data := range [][]byte{[]byte("<svg onload='alert(1)'/>"), []byte("not an image")} {
		if _, err := avatarPNG(data); err == nil {
			t.Fatal("accepted invalid image")
		}
	}
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	if _, err := avatarPNG(b.Bytes()); err != nil {
		t.Fatal(err)
	}
	b.Reset()
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2049, 1)))
	if _, err := avatarPNG(b.Bytes()); err == nil {
		t.Fatal("accepted oversized dimensions")
	}
}

// Uses an isolated schema, never the application's player records.
func TestSettingsIntegration(t *testing.T) {
	dsn := os.Getenv("SETTINGS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SETTINGS_TEST_DATABASE_URL for isolated database integration tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("settings_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	files, _ := filepath.Glob("../../migrations/*.up.sql")
	for _, path := range files {

		data, _ := os.ReadFile(path)
		if _, err = pool.Exec(ctx, string(data)); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("OldPassword123!"), bcrypt.DefaultCost)
	var id int64
	if err = pool.QueryRow(ctx, "INSERT INTO players(login,display_name,password_hash) VALUES('settings_user','Original',$1) RETURNING id", string(hash)).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "INSERT INTO levels(name,author,difficulty,position,game_id,verifier) VALUES('Test','Creator','Extreme',1,12345,'Original')"); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.LoadHTMLGlob("../../templates/*.html")
	router.Use(func(c *gin.Context) {
		if c.GetHeader("X-Test-Auth") == "yes" {
			c.Set("accountID", id)
		}
		c.Next()
	})
	RegisterSettings(router, pool)
	post := func(path string, values url.Values, expected int) {
		t.Helper()
		req := httptest.NewRequest("POST", path, strings.NewReader(values.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Test-Auth", "yes")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != expected {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/settings", nil))
	if w.Code != 401 {
		t.Fatal("anonymous settings access")
	}
	req := httptest.NewRequest("GET", "/settings", nil)
	req.Header.Set("X-Test-Auth", "yes")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Current password") {
		t.Fatal("settings page failed")
	}
	post("/settings/nickname", url.Values{"display_name": {"Renamed"}}, 303)
	var nick, verifier string
	var verifierID int64
	if err = pool.QueryRow(ctx, "SELECT p.display_name,l.verifier,l.verifier_player_id FROM players p JOIN levels l ON l.verifier_player_id=p.id WHERE p.id=$1", id).Scan(&nick, &verifier, &verifierID); err != nil || nick != "Renamed" || verifier != "Renamed" || verifierID != id {
		t.Fatalf("rename not synchronized: %v", err)
	}
	var recordNick string
	if err = pool.QueryRow(ctx, "SELECT p.display_name FROM player_completions c JOIN players p ON p.id=c.player_id WHERE c.player_id=$1", id).Scan(&recordNick); err != nil || recordNick != "Renamed" {
		t.Fatal("completion nickname not synchronized")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, _ := writer.CreateFormFile("avatar", "avatar.png")
	png.Encode(file, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	writer.Close()
	req = httptest.NewRequest("POST", "/settings/avatar", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Test-Auth", "yes")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 303 {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", fmt.Sprintf("/players/%d/avatar", id), nil))
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" {
		t.Fatal("public avatar not available")
	}
	post("/settings/password", url.Values{"current_password": {"wrong"}, "new_password": {"NewPassword123!"}, "confirm_password": {"NewPassword123!"}}, 400)
	var unchanged string
	pool.QueryRow(ctx, "SELECT password_hash FROM players WHERE id=$1", id).Scan(&unchanged)
	if unchanged != string(hash) {
		t.Fatal("wrong current password changed hash")
	}
	pool.Exec(ctx, "INSERT INTO player_sessions(token_hash,player_id,expires_at) VALUES($1,$2,now()+interval '1 day')", []byte("test-session"), id)
	post("/settings/password", url.Values{"current_password": {"OldPassword123!"}, "new_password": {"NewPassword123!"}, "confirm_password": {"NewPassword123!"}}, 303)
	var updated string
	pool.QueryRow(ctx, "SELECT password_hash FROM players WHERE id=$1", id).Scan(&updated)
	if !passwordMatches(updated, "NewPassword123!") || passwordMatches(updated, "OldPassword123!") {
		t.Fatal("password not changed")
	}
	var sessions int
	pool.QueryRow(ctx, "SELECT count(*) FROM player_sessions WHERE player_id=$1", id).Scan(&sessions)
	if sessions != 0 {
		t.Fatal("sessions not revoked")
	}
}
