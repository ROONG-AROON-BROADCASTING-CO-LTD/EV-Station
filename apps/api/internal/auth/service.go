package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrOTPNotConfigured = errors.New("otp delivery is not configured")

type Service struct { repo repository.Repository; secret []byte }
type Claims struct { Subject string `json:"sub"`; Role domain.UserRole `json:"role"`; ExpiresAt int64 `json:"exp"` }

func New(repo repository.Repository, secret string) *Service { return &Service{repo: repo, secret: []byte(secret)} }
func (s *Service) Register(ctx context.Context, email, displayName, password string, role domain.UserRole) (domain.User, error) {
	if role != domain.RoleSuperAdmin && role != domain.RoleAdmin && role != domain.RoleSales && role != domain.RoleCustomer { return domain.User{}, errors.New("invalid role") }
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost); if err != nil { return domain.User{}, err }
	return s.repo.CreateUser(ctx, domain.User{ID: uuid.New(), Email: strings.ToLower(strings.TrimSpace(email)), DisplayName: strings.TrimSpace(displayName), Role: role, IsActive: true, CreatedAt: time.Now().UTC()}, string(hash))
}
func (s *Service) Login(ctx context.Context, email, password string) (domain.User, string, error) {
	user, hash, err := s.repo.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email))); if err != nil || !user.IsActive || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil { return domain.User{}, "", ErrInvalidCredentials }
	return user, s.sign(Claims{Subject:user.ID.String(),Role:user.Role,ExpiresAt:time.Now().Add(12*time.Hour).Unix()}), nil
}
func (s *Service) sign(claims Claims) string { header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`)); payload, _ := json.Marshal(claims); encoded := base64.RawURLEncoding.EncodeToString(payload); mac:=hmac.New(sha256.New,s.secret); mac.Write([]byte(header+"."+encoded)); return header+"."+encoded+"."+base64.RawURLEncoding.EncodeToString(mac.Sum(nil)) }
func (s *Service) Verify(token string) (Claims, error) { parts:=strings.Split(token,"."); if len(parts)!=3{return Claims{},ErrInvalidCredentials}; mac:=hmac.New(sha256.New,s.secret);mac.Write([]byte(parts[0]+"."+parts[1]));signature,err:=base64.RawURLEncoding.DecodeString(parts[2]);if err!=nil||!hmac.Equal(signature,mac.Sum(nil)){return Claims{},ErrInvalidCredentials};payload,err:=base64.RawURLEncoding.DecodeString(parts[1]);if err!=nil{return Claims{},ErrInvalidCredentials};var claims Claims;if json.Unmarshal(payload,&claims)!=nil||claims.ExpiresAt<time.Now().Unix(){return Claims{},ErrInvalidCredentials};if _,err:=uuid.Parse(claims.Subject);err!=nil{return Claims{},ErrInvalidCredentials};return claims,nil }
