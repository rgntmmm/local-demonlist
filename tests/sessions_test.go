package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"local-demonlist/internal/handlers"
	"local-demonlist/internal/middleware"
)

func TestSessionRoutesRejectMissingAndMalformedCookies(t *testing.T) {
	for _, token := range []string{"", "short", strings.Repeat("!", 43)} {
		r := gin.New()
		handlers.RegisterDatabasePages(r, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/session", nil)
		req.AddCookie(&http.Cookie{Name: "local_demonlist_session", Value: token})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", w.Code)
		}
	}
}

func TestSessionLogoutAndCrossOriginProtection(t *testing.T) {
	r := gin.New()
	handlers.RegisterDatabasePages(r, nil)
	for _, crossSite := range []bool{false, true} {
		req := httptest.NewRequest(http.MethodDelete, "/api/session", nil)
		if crossSite {
			req.Header.Set("Sec-Fetch-Site", "cross-site")
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if crossSite {
			if w.Code != http.StatusForbidden {
				t.Fatalf("cross-site logout returned %d", w.Code)
			}
			continue
		}
		if w.Code != http.StatusNoContent {
			t.Fatalf("logout returned %d", w.Code)
		}
		cookies := w.Result().Cookies()
		if len(cookies) != 1 || cookies[0].Name != "local_demonlist_session" || cookies[0].MaxAge != -1 || !cookies[0].HttpOnly {
			t.Fatal("expected session cookie deletion")
		}
	}
}

func TestCurrentSessionResponse(t *testing.T) {
	r := gin.New()
	r.GET("/api/session", func(c *gin.Context) {
		c.Set("accountID", int64(42))
		c.Set("session", gin.H{
			"player_id": int64(42), "login": "player", "display_name": "Player",
			"role": "admin", "expires_at": time.Now().Add(time.Hour),
		})
	}, middleware.RequireSession(), handlers.CurrentSessionHandler)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/session", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"player_id":42`) ||
		!strings.Contains(w.Body.String(), `"role":"admin"`) || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unexpected session response: %d %s", w.Code, w.Body.String())
	}
}
