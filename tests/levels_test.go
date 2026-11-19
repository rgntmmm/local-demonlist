package tests

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"local-demonlist/internal/handlers"
)

func TestLevelManagementPermissions(t *testing.T) {
	for _, role := range []string{"", "player"} {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			if role != "" {
				c.Set("accountID", int64(1))
				c.Set("accountRole", role)
			}
		})
		handlers.RegisterDatabasePages(r, nil)
		for _, path := range []string{"/levels", "/levels/1/edit", "/levels/1/archive", "/levels/1/delete", "/admin/completion-requests/1/review"} {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("POST", path, nil))
			want := http.StatusUnauthorized
			if role == "player" {
				want = http.StatusForbidden
			}
			if w.Code != want {
				t.Fatalf("%s %s: %d, want %d", role, path, w.Code, want)
			}
		}
	}
}

func TestLevelManagementDatabase(t *testing.T) {
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
	fixture, err := os.ReadFile("../seeds/development.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(fixture)); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("A", 43)
	hash := sha256.Sum256([]byte(token))
	var adminID int64
	err = pool.QueryRow(ctx, "INSERT INTO players(login,display_name,password_hash,role) VALUES ('admin','Admin','test','admin') RETURNING id").Scan(&adminID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "INSERT INTO player_sessions VALUES ($1,$2,now()+interval '1 hour')", hash[:], adminID); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.LoadHTMLGlob("../templates/*.html")
	handlers.RegisterDatabasePages(r, pool)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "local_demonlist_session", Value: token})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	if w := request("GET", "/archive", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "No archived levels") || strings.Contains(w.Body.String(), `href="/archive">Archive</a>`) {
		t.Fatal("empty archive navigation")
	}
	var nextGameID atomic.Int64
	form := func(position int) string {
		return url.Values{"game_id": {fmt.Sprint(nextGameID.Add(1))}, "name": {"New level"}, "author": {"Author"}, "difficulty": {"Extreme"}, "position": {fmt.Sprint(position)}}.Encode()
	}
	for _, path := range []string{"/levels/new", "/levels/1/edit"} {
		if w := request("GET", path, ""); w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	if w := request("POST", "/levels", form(99)); w.Code != 400 {
		t.Fatalf("out-of-range returned %d", w.Code)
	}
	if w := request("POST", "/levels", form(2)); w.Code != 303 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	if w := request("POST", "/levels/1/edit", form(4)); w.Code != 303 {
		t.Fatalf("move down: %d", w.Code)
	}
	if w := request("POST", "/levels/1/edit", form(1)); w.Code != 303 {
		t.Fatalf("move up: %d", w.Code)
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if w := request("POST", "/levels", form(1)); w.Code != 303 {
				t.Errorf("concurrent create: %d %s", w.Code, w.Body.String())
			}
		}()
	}
	wg.Wait()
	for position := 9; position <= 31; position++ {
		if w := request("POST", "/levels", form(position)); w.Code != 303 {
			t.Fatalf("position %d: %d", position, w.Code)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO player_completions(player_id,level_id,status,reviewed_by,reviewed_at)
		VALUES ($1,1,'approved',$1,now())`, adminID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if w := request("POST", "/levels/1/archive", ""); w.Code != 303 {
			t.Fatalf("archive: %d", w.Code)
		}
	}
	if w := request("GET", "/archive", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "Return to list") || !strings.Contains(w.Body.String(), `href="/levels/1"`) || !strings.Contains(w.Body.String(), `href="/archive">Archive</a>`) {
		t.Fatalf("archive page: %d %s", w.Code, w.Body.String())
	}
	guest := httptest.NewRecorder()
	r.ServeHTTP(guest, httptest.NewRequest("GET", "/archive", nil))
	if guest.Code != 200 || strings.Contains(guest.Body.String(), "Return to list") {
		t.Fatal("guest archive access")
	}
	guest = httptest.NewRecorder()
	r.ServeHTTP(guest, httptest.NewRequest("POST", "/levels/1/restore", nil))
	if guest.Code != 401 {
		t.Fatal("guest restored level")
	}
	if _, err := pool.Exec(ctx, "UPDATE players SET role='player' WHERE id=$1", adminID); err != nil {
		t.Fatal(err)
	}
	if w := request("POST", "/levels/1/restore", ""); w.Code != 403 {
		t.Fatal("ordinary player restored level")
	}
	if _, err := pool.Exec(ctx, "UPDATE players SET role='admin' WHERE id=$1", adminID); err != nil {
		t.Fatal(err)
	}
	var restoreWG sync.WaitGroup
	for i := 0; i < 2; i++ {
		restoreWG.Add(1)
		go func() {
			defer restoreWG.Done()
			if w := request("POST", "/levels/1/restore", ""); w.Code != 303 {
				t.Errorf("restore: %d", w.Code)
			}
		}()
	}
	restoreWG.Wait()
	var restoredPosition, preservedCompletions int
	if err := pool.QueryRow(ctx, "SELECT position FROM levels WHERE id=1 AND archived_at IS NULL").Scan(&restoredPosition); err != nil || restoredPosition != 31 {
		t.Fatalf("restored placement: %d %v", restoredPosition, err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM player_completions WHERE level_id=1").Scan(&preservedCompletions); err != nil || preservedCompletions != 1 {
		t.Fatal("completion lost during restore")
	}
	if w := request("GET", "/archive", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "No archived levels") || strings.Contains(w.Body.String(), `href="/archive">Archive</a>`) {
		t.Fatal("archive link remains after final restoration")
	}
	if w := request("POST", "/levels/999999/restore", ""); w.Code != 404 {
		t.Fatal("missing level restoration")
	}
	if w := request("POST", "/levels/1/archive", ""); w.Code != 303 {
		t.Fatal("rearchive failed")
	}
	var count, min, max, points, completions int
	err = pool.QueryRow(ctx, "SELECT count(*),min(position),max(position) FROM levels WHERE archived_at IS NULL").Scan(&count, &min, &max)
	if err != nil || count != 30 || min != 1 || max != 30 {
		t.Fatalf("positions: %d %d %d %v", count, min, max, err)
	}
	err = pool.QueryRow(ctx, "SELECT total_points,completed_count FROM player_leaderboard WHERE player_id=$1", adminID).Scan(&points, &completions)
	if err != nil || points != 0 || completions != 1 {
		t.Fatalf("archived score: %d %d %v", points, completions, err)
	}
	for _, path := range []string{"/levels/1", fmt.Sprintf("/players/%d", adminID)} {
		if w := request("GET", path, ""); w.Code != 200 || !strings.Contains(w.Body.String(), "Archived") {
			t.Fatalf("archive page %s: %d %s", path, w.Code, w.Body.String())
		}
	}
	if w := request("POST", "/levels/1/edit", form(1)); w.Code != 404 {
		t.Fatalf("editing archived level: %d", w.Code)
	}

	var completionID int64
	var submitterID int64
	if err := pool.QueryRow(ctx, "INSERT INTO players(login,display_name,password_hash) VALUES('review_submitter','Submitter','!test') RETURNING id").Scan(&submitterID); err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, "INSERT INTO player_completions(player_id,level_id) VALUES ($1,4) RETURNING id", submitterID).Scan(&completionID)
	if err != nil {
		t.Fatal(err)
	}
	if w := request("GET", "/admin/completion-requests", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "Awaiting review: 1") {
		t.Fatalf("pending queue: %d %s", w.Code, w.Body.String())
	}
	reviewPath := fmt.Sprintf("/admin/completion-requests/%d/review", completionID)
	if w := request("POST", reviewPath, "decision=invalid"); w.Code != 400 {
		t.Fatalf("invalid decision: %d", w.Code)
	}
	var reviews sync.WaitGroup
	codes := make(chan int, 2)
	for _, decision := range []string{"approved", "rejected"} {
		reviews.Add(1)
		go func(decision string) {
			defer reviews.Done()
			codes <- request("POST", reviewPath, "decision="+decision).Code
		}(decision)
	}
	reviews.Wait()
	close(codes)
	results := map[int]int{}
	for code := range codes {
		results[code]++
	}
	if results[303] != 1 || results[409] != 1 {
		t.Fatalf("concurrent review: %v", results)
	}
	var reviewer int64
	var reviewed bool
	if err := pool.QueryRow(ctx, "SELECT reviewed_by,reviewed_at IS NOT NULL FROM player_completions WHERE id=$1", completionID).Scan(&reviewer, &reviewed); err != nil || reviewer != adminID || !reviewed {
		t.Fatalf("review audit: %d %v %v", reviewer, reviewed, err)
	}
	if w := request("GET", "/admin/completion-requests", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "No submissions") {
		t.Fatalf("empty queue: %d", w.Code)
	}
	edit := url.Values{"game_id": {"9000000000000002"}, "name": {"Updated"}, "author": {"New author"}, "difficulty": {"Hard"},
		"position": {"2"}, "verifier": {"Verifier"}, "icon": {"https://example.com/icon.png"},
		"video_url": {"https://example.com/video"}}
	if w := request("POST", "/levels/2/edit", edit.Encode()); w.Code != 303 {
		t.Fatalf("edit metadata: %d %s", w.Code, w.Body.String())
	}
	var name, author, verifier, icon string
	err = pool.QueryRow(ctx, "SELECT name,author,verifier,level_icon FROM levels WHERE id=2").Scan(&name, &author, &verifier, &icon)
	if err != nil || name != "Updated" || author != "New author" || verifier != "Verifier" || icon != "https://example.com/icon.png" {
		t.Fatalf("metadata not saved: %s %s %s %s %v", name, author, verifier, icon, err)
	}
	if w := request("POST", "/levels/1/delete", ""); w.Code != 409 {
		t.Fatalf("delete with completion: %d", w.Code)
	}
	// Remove the verifier assignment and its automatic completion before deletion.
	edit.Set("verifier", "")
	if w := request("POST", "/levels/2/edit", edit.Encode()); w.Code != 303 {
		t.Fatalf("clear verifier: %d", w.Code)
	}
	if w := request("POST", "/levels/2/delete", ""); w.Code != 303 {
		t.Fatalf("delete active: %d %s", w.Code, w.Body.String())
	}
	if w := request("POST", "/levels/2/delete", ""); w.Code != 404 {
		t.Fatalf("repeat delete: %d", w.Code)
	}
	if w := request("POST", "/levels/3/archive", ""); w.Code != 303 {
		t.Fatalf("archive before delete: %d", w.Code)
	}
	if w := request("POST", "/levels/3/delete", ""); w.Code != 303 {
		t.Fatalf("delete archived: %d", w.Code)
	}
	err = pool.QueryRow(ctx, "SELECT count(*),min(position),max(position) FROM levels WHERE archived_at IS NULL").Scan(&count, &min, &max)
	if err != nil || count != 28 || min != 1 || max != 28 {
		t.Fatalf("positions after delete: %d %d %d %v", count, min, max, err)
	}
	req := httptest.NewRequest("POST", "/levels", strings.NewReader(form(1)))
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.AddCookie(&http.Cookie{Name: "local_demonlist_session", Value: token})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatalf("cross-site write: %d", w.Code)
	}
}
