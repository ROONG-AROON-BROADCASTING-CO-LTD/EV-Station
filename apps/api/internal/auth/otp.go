package auth

import (
	"crypto/rand"
	"fmt"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

type OTPService struct {
	mu sync.Mutex
	codes map[string]otpRecord
	host, port, username, password, from string
	ttl time.Duration
}
type otpRecord struct { code string; expiresAt time.Time }
func NewOTPService(host, port, username, password, from string, ttl time.Duration) *OTPService {
	return &OTPService{codes: map[string]otpRecord{}, host: strings.TrimSpace(host), port: strings.TrimSpace(port), username: strings.TrimSpace(username), password: password, from: strings.TrimSpace(from), ttl: ttl}
}
func (s *OTPService) Request(email string) error {
	if s.host == "" || s.port == "" || s.from == "" { return ErrOTPNotConfigured }
	var raw [3]byte
	if _, err := rand.Read(raw[:]); err != nil { return err }
	code := fmt.Sprintf("%06d", (int(raw[0])<<16|int(raw[1])<<8|int(raw[2]))%1000000)
	email = strings.ToLower(strings.TrimSpace(email))
	s.mu.Lock(); s.codes[email] = otpRecord{code: code, expiresAt: time.Now().Add(s.ttl)}; s.mu.Unlock()
	address := s.host + ":" + s.port
	body := "To: " + email + "\r\nFrom: " + s.from + "\r\nSubject: RBC EV Station verification code\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nYour verification code is " + code + ". It expires in " + s.ttl.String() + "."
	var auth smtp.Auth
	if s.username != "" { auth = smtp.PlainAuth("", s.username, s.password, s.host) }
	return smtp.SendMail(address, auth, s.from, []string{email}, []byte(body))
}
func (s *OTPService) Verify(email, code string) bool {
	email = strings.ToLower(strings.TrimSpace(email)); s.mu.Lock(); defer s.mu.Unlock()
	record, ok := s.codes[email]
	if !ok || time.Now().After(record.expiresAt) || record.code != strings.TrimSpace(code) { return false }
	delete(s.codes, email); return true
}
