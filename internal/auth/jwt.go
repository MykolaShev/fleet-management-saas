package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/MykolaShev/fleet-management-saas/internal/domain"
)

// ErrInvalidToken covers every way a token can fail to validate (bad
// signature, expired, malformed) — callers don't need to distinguish why,
// just that the request isn't authenticated.
var ErrInvalidToken = errors.New("invalid or expired token")

// Claims is what our own session JWTs carry. Deliberately minimal: enough
// to resolve "who is calling, and for which tenant" without a database
// round-trip on every request.
type Claims struct {
	UserID   uuid.UUID       `json:"user_id"`
	TenantID uuid.UUID       `json:"tenant_id"`
	Role     domain.UserRole `json:"role"`
	jwt.RegisteredClaims
}

// JWTIssuer issues and validates our own session tokens, HMAC-signed with
// a shared secret. This is entirely our own trust domain — separate from
// Google's tokens, which we only ever see once, at login (see
// auth.ParseIDToken) — so there's no need for asymmetric keys here.
type JWTIssuer struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTIssuer(secret string, ttl time.Duration) *JWTIssuer {
	return &JWTIssuer{secret: []byte(secret), ttl: ttl}
}

func (j *JWTIssuer) Issue(userID, tenantID uuid.UUID, role domain.UserRole) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		TenantID: tenantID,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secret)
}

func (j *JWTIssuer) Parse(tokenString string) (*Claims, error) {
	var claims Claims
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (interface{}, error) {
		return j.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return &claims, nil
}
