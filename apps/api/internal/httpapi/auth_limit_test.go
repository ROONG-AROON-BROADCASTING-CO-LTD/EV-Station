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
	now = now.Add(15 * time.Minute)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(`{"email":" Test@Example.com "}`))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("attempt after expiry: %d, want 401", w.Code)
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

func TestLoginRecoversAfterRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	l := newAuthLimiter()
	now := time.Now()
	l.now = func() time.Time { return now }
	r := gin.New()
	r.POST("/api/v1/auth/login", l.middleware(), func(c *gin.Context) {
		var input struct {
			Password string `json:"password"`
		}
		_ = c.ShouldBindJSON(&input)
		if input.Password == "correct" {
			c.Status(http.StatusOK)
			return
		}
		c.Status(http.StatusUnauthorized)
	})
	r.POST("/api/v1/auth/register/request-otp", l.middleware(), func(c *gin.Context) {
		c.Status(http.StatusAccepted)
	})
	r.POST("/api/v1/auth/password-reset/request-otp", l.middleware(), func(c *gin.Context) {
		c.Status(http.StatusAccepted)
	})
	attempt := func(password string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{"email":"qa@example.invalid","password":"`+password+`"}`))
		r.ServeHTTP(w, req)
		return w.Code
	}
	for i := 0; i < 10; i++ {
		if got := attempt("wrong"); got != http.StatusUnauthorized {
			t.Fatalf("wrong attempt %d: %d", i+1, got)
		}
	}
	if got := attempt("correct"); got != http.StatusTooManyRequests {
		t.Fatalf("correct password while limited: %d", got)
	}
	otp := httptest.NewRecorder()
	r.ServeHTTP(otp, httptest.NewRequest("POST", "/api/v1/auth/register/request-otp", strings.NewReader(`{"email":"qa@example.invalid"}`)))
	if otp.Code != http.StatusAccepted {
		t.Fatalf("OTP request while login is limited: %d", otp.Code)
	}
	resetOTP := httptest.NewRecorder()
	r.ServeHTTP(resetOTP, httptest.NewRequest("POST", "/api/v1/auth/password-reset/request-otp", strings.NewReader(`{"email":"qa@example.invalid"}`)))
	if resetOTP.Code != http.StatusAccepted {
		t.Fatalf("password reset OTP request while login is limited: %d", resetOTP.Code)
	}
	now = now.Add(15 * time.Minute)
	if got := attempt("wrong"); got != http.StatusUnauthorized {
		t.Fatalf("wrong password after expiry: %d", got)
	}
	if got := attempt("correct"); got != http.StatusOK {
		t.Fatalf("correct password after expiry: %d", got)
	}
	for i := 0; i < 10; i++ {
		if got := attempt("wrong"); got != http.StatusUnauthorized {
			t.Fatalf("attempt after successful login %d: %d", i+1, got)
		}
	}
}
