package tests

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"local-demonlist/internal/handlers"
)

func TestPreviewPages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Static("/static", "../static")
	r.LoadHTMLGlob("../templates/*.html")
	handlers.RegisterPreviewPages(r)

	for _, tc := range []struct {
		path   string
		status int
		body   string
	}{
		{"/", 302, "/demonlist"},
		{"/demonlist", 200, "Sample Level A"},
		{"/demonlist?query=AnotherCreator", 200, "Sample Level B"},
		{"/demonlist?query=missing", 200, "No matching levels"},
		{"/levels/new", 200, "disabled aria-describedby"},
		{"/levels/1", 200, "ExamplePlayer"},
		{"/levels/2", 200, "No verified completions yet"},
		{"/players/1", 200, "1000"},
		{"/leaderboard", 200, "href=\"/players/1\""},
		{"/levels/999", 404, "Level not found"},
		{"/levels/invalid", 404, "Level not found"},
		{"/players/999", 404, "Player not found"},
		{"/static/styles.css", 200, ".level-form"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.body) {
				t.Fatalf("GET %s: status %d, expected %d and body containing %q", tc.path, w.Code, tc.status, tc.body)
			}
			if tc.status == 200 && tc.path != "/static/styles.css" {
				if !strings.Contains(w.Header().Get("Content-Type"), "text/html") || !strings.Contains(w.Body.String(), `/static/styles.css`) {
					t.Fatal("expected HTML page with stylesheet link")
				}
			}
		})
	}
}
