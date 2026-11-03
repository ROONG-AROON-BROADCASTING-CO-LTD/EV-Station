package auth

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestOTPAttemptsExpiryAndOneTimeUse(t *testing.T) {
	s := NewOTPService("localhost", "25", "", "", "test@example.invalid", time.Minute)
	s.codes["test@example.invalid"] = otpRecord{code: "123456", expiresAt: time.Now().Add(time.Minute)}
	for i := 0; i < 5; i++ {
		if s.Verify("test@example.invalid", "wrong") {
			t.Fatal("invalid OTP accepted")
		}
	}
	if s.Verify("test@example.invalid", "123456") {
		t.Fatal("exhausted OTP accepted")
	}
	s.codes["test@example.invalid"] = otpRecord{code: "123456", expiresAt: time.Now().Add(-time.Second)}
	if s.Verify("test@example.invalid", "123456") {
		t.Fatal("expired OTP accepted")
	}
	if len(s.codes) != 0 {
		t.Fatal("expired entry retained")
	}
	s.codes["test@example.invalid"] = otpRecord{code: "123456", expiresAt: time.Now().Add(time.Minute)}
	if !s.Verify("TEST@example.invalid", "123456") || s.Verify("test@example.invalid", "123456") {
		t.Fatal("one-time OTP validation failed")
	}
}

func TestSMTPRequestCancellationAndFailedCodeRemoval(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, e := listener.Accept()
		if e != nil {
			return
		}
		defer conn.Close()
		var b [1]byte
		conn.Read(b[:])
	}()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	s := NewOTPService(host, port, "", "", "test@example.invalid", time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err = s.RequestContext(ctx, "test@example.invalid"); err == nil {
		t.Fatal("stalled SMTP succeeded")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("SMTP ignored cancellation")
	}
	if len(s.codes) != 0 {
		t.Fatal("undelivered code retained")
	}
	<-done
}
