package httpapi

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUnrelatedAuthSourcesDoNotBlockVictim(t *testing.T) {
	gin.SetMode(gin.TestMode)
	l := newAuthLimiter()
	r := gin.New()
	r.POST("/auth/login", l.middleware(), func(c *gin.Context) { c.Status(401) })
	for i := 0; i < 120; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(fmt.Sprintf(`{"email":"fake%d@example.invalid"}`, i)))
		req.RemoteAddr = "192.0.2.1:1234"
		r.ServeHTTP(w, req)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(`{"email":"victim@example.invalid"}`))
	req.RemoteAddr = "192.0.2.2:1234"
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("unrelated source blocked victim: %d", w.Code)
	}
}

func TestAuthSourceHeaderRequiresTrustedProxy(t *testing.T) {
	l := newAuthLimiter()
	l.trustProxies("172.20.0.0/16")
	req := httptest.NewRequest("POST", "/auth/login", nil)
	req.RemoteAddr = "192.0.2.1:1000"
	req.Header.Set("X-RBC-Client-IP", "192.0.2.9")
	req.Header.Set("X-Forwarded-For", "192.0.2.8")
	if got := l.requestSource(req); got != "192.0.2.1" {
		t.Fatal("untrusted spoof accepted", got)
	}
	req.RemoteAddr = "172.20.0.2:1000"
	if got := l.requestSource(req); got != "192.0.2.9" {
		t.Fatal("trusted identity ignored", got)
	}
	req.Header.Set("X-RBC-Client-IP", "invalid")
	if got := l.requestSource(req); got != "172.20.0.2" {
		t.Fatal("invalid forwarded address accepted", got)
	}
}

func TestOwnerBootstrapIsExplicitAndEmailBound(t *testing.T) {
	h := &Handler{}
	if h.mayBootstrapOwner("first@example.invalid", false) {
		t.Fatal("public first registrant became owner")
	}
	h.bootstrapOwnerEmail = "approved@example.invalid"
	if h.mayBootstrapOwner("attacker@example.invalid", false) {
		t.Fatal("wrong address became owner")
	}
	if !h.mayBootstrapOwner(" APPROVED@example.invalid ", false) {
		t.Fatal("configured bootstrap denied")
	}
	if h.mayBootstrapOwner("approved@example.invalid", true) {
		t.Fatal("bootstrap after existing owner")
	}
}

func TestSuccessfulLoginsDoNotAccumulateAccountLockout(t *testing.T) {
	l := newAuthLimiter()
	r := gin.New()
	r.Group("/api/v1").POST("/auth/login", l.middleware(), func(c *gin.Context) { c.Status(200) })
	for i := 0; i < 25; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{"email":"valid@example.invalid"}`)))
		if w.Code != 200 {
			t.Fatalf("valid login %d blocked: %d", i, w.Code)
		}
	}
}
