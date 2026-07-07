// Package auth handles password hashing, JWT issuing/parsing and the HTTP
// middleware that protects authenticated routes.
package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/guilherme/help-party/internal/httpx"
)

type ctxKey string

const userCtxKey ctxKey = "user"

// Claims is our JWT payload.
type Claims struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	IsAdmin bool   `json:"is_admin"`
	jwt.RegisteredClaims
}

// User is the minimal authenticated principal stored in the request context.
type User struct {
	ID      uuid.UUID
	Email   string
	Name    string
	IsAdmin bool
}

// Service issues and validates tokens using a shared secret.
type Service struct {
	secret []byte
	ttl    time.Duration
}

func NewService(secret string, ttl time.Duration) *Service {
	return &Service{secret: []byte(secret), ttl: ttl}
}

// HashPassword returns a bcrypt hash of the plaintext password.
func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword reports whether the plaintext matches the stored hash.
func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// Issue creates a signed JWT for the given user.
func (s *Service) Issue(u User) (string, error) {
	now := time.Now()
	claims := Claims{
		Email:   u.Email,
		Name:    u.Name,
		IsAdmin: u.IsAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(s.secret)
}

// Parse validates a token string and returns the embedded user.
func (s *Service) Parse(tokenStr string) (User, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil {
		return User{}, err
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return User{}, errors.New("invalid subject")
	}
	return User{ID: id, Email: claims.Email, Name: claims.Name, IsAdmin: claims.IsAdmin}, nil
}

// RequireAuth is middleware that rejects requests without a valid Bearer token
// and injects the authenticated user into the request context.
func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			httpx.Error(w, http.StatusUnauthorized, "missing bearer token")
			return
		}
		user, err := s.Parse(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "invalid token")
			return
		}
		ctx := context.WithValue(r.Context(), userCtxKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin is middleware that rejects non-admin users. It must be chained
// after RequireAuth.
func (s *Service) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := FromContext(r.Context())
		if !ok || !u.IsAdmin {
			httpx.Error(w, http.StatusForbidden, "apenas administradores")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// FromContext returns the authenticated user previously set by RequireAuth.
func FromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userCtxKey).(User)
	return u, ok
}
