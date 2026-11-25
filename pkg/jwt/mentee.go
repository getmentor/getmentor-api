package jwt

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// MenteeTokenType is the token_type claim every mentee session token carries.
	MenteeTokenType = "mentee_session"
	// MenteeAudience is the aud claim every mentee session token carries.
	MenteeAudience = "getmentor-mentee"

	// menteeKeyLabel derives the mentee signing key from JWT_SECRET. Mentor and admin
	// tokens are signed with JWT_SECRET itself, so a token from one realm never has a
	// valid signature in the other, even though no new secret has to be configured.
	menteeKeyLabel = "getmentor:mentee-session:v1"
)

// MenteeClaims are the claims of a mentee session token. The email is deliberately
// not included: the session only names the mentee row, whose email is read from the DB.
type MenteeClaims struct {
	TokenType      string `json:"token_type"`
	SessionVersion int    `json:"sv"`
	jwt.RegisteredClaims
}

// MenteeTokenManager issues and validates mentee session tokens. It is a separate
// realm from TokenManager (mentor/admin): its own key, audience and token type.
type MenteeTokenManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

// NewMenteeTokenManager creates a manager whose key is derived from the shared JWT secret.
func NewMenteeTokenManager(jwtSecret, issuer string, ttlHours int) *MenteeTokenManager {
	mac := hmac.New(sha256.New, []byte(jwtSecret))
	mac.Write([]byte(menteeKeyLabel))
	return &MenteeTokenManager{
		secret: mac.Sum(nil),
		issuer: issuer,
		ttl:    time.Duration(ttlHours) * time.Hour,
	}
}

// GenerateToken signs a session token for the mentee row id and its current session version.
func (tm *MenteeTokenManager) GenerateToken(menteeID string, sessionVersion int) (string, error) {
	if menteeID == "" || sessionVersion < 1 {
		return "", ErrInvalidClaim
	}
	now := time.Now()
	claims := MenteeClaims{
		TokenType:      MenteeTokenType,
		SessionVersion: sessionVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   menteeID,
			Issuer:    tm.issuer,
			Audience:  jwt.ClaimStrings{MenteeAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tm.ttl)),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(tm.secret)
	if err != nil {
		return "", fmt.Errorf("failed to sign mentee token: %w", err)
	}
	return signed, nil
}

// ValidateToken checks signature, algorithm, issuer, audience and expiry, and then
// requires every realm claim. Unlike the mentor/admin realms, nothing is optional:
// a token missing token_type, sub, sv or exp is refused.
func (tm *MenteeTokenManager) ValidateToken(tokenString string) (*MenteeClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&MenteeClaims{},
		func(*jwt.Token) (interface{}, error) { return tm.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithAudience(MenteeAudience),
		jwt.WithIssuer(tm.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	claims, ok := token.Claims.(*MenteeClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidClaim
	}
	if claims.TokenType != MenteeTokenType || claims.Subject == "" || claims.SessionVersion < 1 {
		return nil, ErrInvalidClaim
	}
	return claims, nil
}

// GetExpirationTime returns the session lifetime.
func (tm *MenteeTokenManager) GetExpirationTime() time.Duration {
	return tm.ttl
}
