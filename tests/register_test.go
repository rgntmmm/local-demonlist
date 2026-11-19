package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"local-demonlist/internal/handlers"
)

func TestRegisterRejectsInvalidInputBeforeDatabase(t *testing.T) {
	for _, body := range []string{
		`{`,
		`{}`,
		`{"login":"bad-name","display_name":"Player","password":"password123"}`,
		`{"login":"player","display_name":"   ","password":"password123"}`,
		`{"login":"player","display_name":"Player","password":"short"}`,
		`{"login":"player","display_name":"Player","password":"` + strings.Repeat("я", 37) + `"}`,
		`{"login":"player","display_name":"` + strings.Repeat("a", 5000) + `","password":"password123"}`,
	} {
		r := gin.New()
		r.POST("/api/register", handlers.RegisterHandler(nil))
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("invalid input returned %d, want 400", w.Code)
		}
	}
}
