package httpapi

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAuthLimiterWindowAndBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	l := newAuthLimiter()
	now := time.Now()
	l.now = func() time.Time { return now }
	r := gin.New()
	r.POST("/auth/login", l.middleware(), func(c *gin.Context) {
		var body struct {
			Email string `json:"email"`
		}
		if c.ShouldBindJSON(&body) != nil || body.Email == "" {
			t.Error("middleware consumed request body")
		}
		c.Status(401)
	})
	for i := 0; i < 11; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(`{"email":" Test@Example.com "}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		want := 401
		if i == 10 {
			want = 429
		}
		if w.Code != want {
			t.Fatalf("attempt %d: %d, want %d", i, w.Code, want)
		}
		if i == 10 && w.Header().Get("Retry-After") == "" {
			t.Fatal("missing retry guidance")
		}
	}
	now = now.Add(16 * time.Minute)
	if !l.allow("/auth/login:test@example.com", 10, 15*time.Minute) {
		t.Fatal("expired bucket did not recover")
	}
}

func TestAuthLimiterBoundsAndIsolation(t *testing.T) {
	l := newAuthLimiter()
	if !l.allow("a", 1, time.Minute) || l.allow("a", 1, time.Minute) || !l.allow("b", 1, time.Minute) {
		t.Fatal("incorrect isolation")
	}
	for i := 0; i < 120; i++ {
		if !l.allow("all", 120, time.Minute) {
			t.Fatal("global ceiling too early")
		}
	}
	if l.allow("all", 120, time.Minute) {
		t.Fatal("global ceiling not enforced")
	}
	r := gin.New()
	r.POST("/auth/login", l.middleware(), func(c *gin.Context) { c.Status(http.StatusOK) })
}
