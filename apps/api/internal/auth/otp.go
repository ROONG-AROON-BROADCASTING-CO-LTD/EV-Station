package auth

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

const maxOTPCodes = 10000

type OTPService struct {
	mu                                   sync.Mutex
	codes                                map[string]otpRecord
	host, port, username, password, from string
	ttl                                  time.Duration
}
type otpRecord struct {
	code      string
	expiresAt time.Time
	attempts  int
}

func NewOTPService(host, port, username, password, from string, ttl time.Duration) *OTPService {
	return &OTPService{codes: map[string]otpRecord{}, host: strings.TrimSpace(host), port: strings.TrimSpace(port), username: username, password: password, from: strings.TrimSpace(from), ttl: ttl}
}
func (s *OTPService) Request(email string) error {
	return s.RequestContext(context.Background(), email)
}
func (s *OTPService) RequestContext(ctx context.Context, email string) error {
	if s.host == "" || s.port == "" || s.from == "" {
		return ErrOTPNotConfigured
	}
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 320 || strings.ContainsAny(email, "\r\n") {
		return errors.New("invalid email")
	}
	value, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return err
	}
	code := fmt.Sprintf("%06d", value.Int64())
	s.mu.Lock()
	for key, record := range s.codes {
		if !time.Now().Before(record.expiresAt) {
			delete(s.codes, key)
		}
	}
	if _, exists := s.codes[email]; !exists && len(s.codes) >= maxOTPCodes {
		s.mu.Unlock()
		return errors.New("verification temporarily unavailable")
	}
	s.codes[email] = otpRecord{code: code, expiresAt: time.Now().Add(s.ttl)}
	s.mu.Unlock()
	body := "To: " + email + "\r\nFrom: " + s.from + "\r\nSubject: RBC EV Station verification code\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nYour verification code is " + code
	if err = s.send(ctx, email, []byte(body)); err != nil {
		s.mu.Lock()
		if current := s.codes[email]; current.code == code {
			delete(s.codes, email)
		}
		s.mu.Unlock()
	}
	return err
}
func (s *OTPService) send(ctx context.Context, to string, body []byte) error {
	// HTTP write timeouts do not cancel SMTP I/O. Bound the entire handshake,
	// delivery and shutdown, and close the socket when the request is canceled.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(s.host, s.port))
	if err != nil {
		return err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		if err = conn.SetDeadline(deadline); err != nil {
			return err
		}
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return err
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err = client.StartTLS(&tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	} else if s.username != "" {
		return errors.New("SMTP authentication requires TLS")
	}
	if s.username != "" {
		if err = client.Auth(smtp.PlainAuth("", s.username, s.password, s.host)); err != nil {
			return err
		}
	}
	if err = client.Mail(s.from); err != nil {
		return err
	}
	if err = client.Rcpt(to); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = writer.Write(body); err != nil {
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}
func (s *OTPService) Verify(email, code string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.codes[email]
	if !ok {
		return false
	}
	if !time.Now().Before(record.expiresAt) {
		delete(s.codes, email)
		return false
	}
	record.attempts++
	if record.code != strings.TrimSpace(code) {
		if record.attempts >= 5 {
			delete(s.codes, email)
		} else {
			s.codes[email] = record
		}
		return false
	}
	delete(s.codes, email)
	return true
}
