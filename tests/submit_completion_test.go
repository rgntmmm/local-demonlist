package tests

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"local-demonlist/internal/handlers"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompletionSubmissionDatabase(t *testing.T) {
	if os.Getenv("RUN_DATABASE_TESTS") != "1" {
		t.Skip("set RUN_DATABASE_TESTS=1 to run against an isolated PostgreSQL schema")
	}
	env, err := godotenv.Read("../.env")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	root, err := pgxpool.New(ctx, env["DATABASE_URL"])
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	schema := fmt.Sprintf("test_levels_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = root.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := root.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	cfg, err := pgxpool.ParseConfig(env["DATABASE_URL"])
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	files, err := filepath.Glob("../migrations/*.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {

		sql, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
	}

	var playerID int64
	if err := pool.QueryRow(ctx, "INSERT INTO players(login,display_name,password_hash) VALUES ('submitter','Submitter','!test') RETURNING id").Scan(&playerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO levels(name,author,difficulty,position,game_id) VALUES ('Test level','Author','Extreme',1,987654),('Second level','Author','Extreme',2,987655)"); err != nil {
		t.Fatal(err)
	}
	role := "player"
	r := gin.New()
	r.LoadHTMLGlob("../templates/*.html")
	r.Use(func(c *gin.Context) {
		if role != "" {
			c.Set("accountID", playerID)
			c.Set("accountRole", role)
		}
	})
	handlers.RegisterDatabasePages(r, pool)
	request := func(method, path string, form url.Values) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.ServeHTTP(w, req)
		return w
	}
	form := url.Values{"level": {"987654"}, "attempts": {"123"}, "contact_type": {"discord"}, "contact": {"test.user"}}
	role = ""
	if w := request("POST", "/completion-requests", form); w.Code != 401 {
		t.Fatalf("guest: %d", w.Code)
	}
	role = "player"
	if w := request("GET", "/completion-requests/new", nil); w.Code != 200 || !strings.Contains(w.Body.String(), "Submit a record") {
		t.Fatalf("form: %d %s", w.Code, w.Body.String())
	}
	if w := request("POST", "/completion-requests", form); w.Code != 400 {
		t.Fatalf("missing video: %d", w.Code)
	}
	form.Set("video_url", "javascript:alert(1)")
	if w := request("POST", "/completion-requests", form); w.Code != 400 {
		t.Fatalf("bad video: %d", w.Code)
	}
	form.Set("video_url", "https://example.com/video")
	form.Set("attempts", "0")
	if w := request("POST", "/completion-requests", form); w.Code != 400 {
		t.Fatalf("attempts: %d", w.Code)
	}
	form.Set("attempts", "123")
	if w := request("POST", "/completion-requests", form); w.Code != 303 {
		t.Fatalf("submit: %d %s", w.Code, w.Body.String())
	}
	if w := request("POST", "/completion-requests", form); w.Code != 409 {
		t.Fatalf("duplicate: %d", w.Code)
	}
	if w := request("GET", "/admin/completion-requests", nil); w.Code != 403 {
		t.Fatalf("private contacts: %d", w.Code)
	}
	role = "admin"
	form.Set("level", "Second level")
	form.Del("video_url")
	form.Set("contact_type", "email")
	form.Set("contact", "invalid")
	if w := request("POST", "/completion-requests", form); w.Code != 400 {
		t.Fatalf("email: %d", w.Code)
	}
	form.Set("contact", "player@example.com")
	if w := request("POST", "/completion-requests", form); w.Code != 303 {
		t.Fatalf("admin: %d %s", w.Code, w.Body.String())
	}
	w := request("GET", "/admin/completion-requests", nil)
	for _, value := range []string{"Submitter", "123", "test.user", "player@example.com", "987654"} {
		if w.Code != 200 || !strings.Contains(w.Body.String(), value) {
			t.Fatalf("queue missing %s: %d %s", value, w.Code, w.Body.String())
		}
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM player_completions WHERE player_id=$1 AND status='pending' AND attempts=123", playerID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("persisted: %d %v", count, err)
	}
	if strings.Contains(w.Body.String(), `name="decision"`) {
		t.Fatal("own submissions expose review controls")
	}
	var ids []int64
	rows, err := pool.Query(ctx, "SELECT id FROM player_completions WHERE player_id=$1 ORDER BY id", playerID)
	if err != nil {
		t.Fatal(err)
	}
	ids, err = pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		for _, decision := range []string{"approved", "rejected"} {
			response := request("POST", fmt.Sprintf("/admin/completion-requests/%d/review", id), url.Values{"decision": {decision}})
			if response.Code != 403 {
				t.Fatalf("self review: %d", response.Code)
			}
		}
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM player_completions WHERE player_id=$1 AND status='pending' AND reviewed_by IS NULL AND reviewed_at IS NULL", playerID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("self review changed records: %d %v", count, err)
	}
	var otherAdminID int64
	if err := pool.QueryRow(ctx, "INSERT INTO players(login,display_name,password_hash,role) VALUES ('reviewer','Reviewer','!test','admin') RETURNING id").Scan(&otherAdminID); err != nil {
		t.Fatal(err)
	}
	playerID = otherAdminID
	w = request("GET", "/admin/completion-requests", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `name="decision"`) {
		t.Fatal("other admin cannot see review controls")
	}
	for i, decision := range []string{"approved", "rejected"} {
		path := fmt.Sprintf("/admin/completion-requests/%d/review", ids[i])
		if response := request("POST", path, url.Values{"decision": {decision}}); response.Code != 303 {
			t.Fatalf("other admin review: %d", response.Code)
		}
		var status string
		var reviewer int64
		if err := pool.QueryRow(ctx, "SELECT status,reviewed_by FROM player_completions WHERE id=$1", ids[i]).Scan(&status, &reviewer); err != nil || status != decision || reviewer != otherAdminID {
			t.Fatalf("review not saved: %s %d %v", status, reviewer, err)
		}
		if response := request("POST", path, url.Values{"decision": {decision}}); response.Code != 409 {
			t.Fatalf("repeat review: %d", response.Code)
		}
	}

}
