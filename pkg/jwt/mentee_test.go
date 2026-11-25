package jwt

import (
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-test-secret-test-secret"

func TestMenteeToken_RoundTrip(t *testing.T) {
	tm := NewMenteeTokenManager(testSecret, "getmentor-api", 1)
	token, err := tm.GenerateToken("mentee-1", 3)
	require.NoError(t, err)

	claims, err := tm.ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, "mentee-1", claims.Subject)
	assert.Equal(t, 3, claims.SessionVersion)
	assert.Equal(t, MenteeTokenType, claims.TokenType)
}

func TestMenteeToken_RejectsMentorAndAdminTokens(t *testing.T) {
	mentorRealm := NewTokenManager(testSecret, "getmentor-api", 1)
	menteeRealm := NewMenteeTokenManager(testSecret, "getmentor-api", 1)

	mentorToken, err := mentorRealm.GenerateToken("mentor-1", 1, "", "Mentor")
	require.NoError(t, err)
	adminToken, err := mentorRealm.GenerateTokenWithRole("moderator-1", 0, "a@example.com", "Admin", "admin")
	require.NoError(t, err)

	_, err = menteeRealm.ValidateToken(mentorToken)
	assert.ErrorIs(t, err, ErrInvalidToken)
	_, err = menteeRealm.ValidateToken(adminToken)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

func TestMenteeToken_RejectedByMentorAndAdminRealm(t *testing.T) {
	mentorRealm := NewTokenManager(testSecret, "getmentor-api", 1)
	menteeToken, err := NewMenteeTokenManager(testSecret, "getmentor-api", 1).GenerateToken("mentee-1", 1)
	require.NoError(t, err)

	// Mentor and admin sessions both validate through TokenManager.ValidateToken.
	_, err = mentorRealm.ValidateToken(menteeToken)
	assert.ErrorIs(t, err, ErrInvalidToken)
}

// signWithMenteeKey signs arbitrary claims with the real mentee key, as an attacker who
// somehow had the key could; validation must still demand every realm claim.
func signWithMenteeKey(t *testing.T, tm *MenteeTokenManager, claims gojwt.Claims) string {
	t.Helper()
	s, err := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims).SignedString(tm.secret)
	require.NoError(t, err)
	return s
}

func TestMenteeToken_RefusesMissingRealmClaims(t *testing.T) {
	tm := NewMenteeTokenManager(testSecret, "getmentor-api", 1)
	now := time.Now()
	valid := func() MenteeClaims {
		return MenteeClaims{
			TokenType:      MenteeTokenType,
			SessionVersion: 1,
			RegisteredClaims: gojwt.RegisteredClaims{
				Subject:   "mentee-1",
				Issuer:    "getmentor-api",
				Audience:  gojwt.ClaimStrings{MenteeAudience},
				IssuedAt:  gojwt.NewNumericDate(now),
				ExpiresAt: gojwt.NewNumericDate(now.Add(time.Hour)),
			},
		}
	}

	cases := map[string]func(c *MenteeClaims){
		"no token_type":        func(c *MenteeClaims) { c.TokenType = "" },
		"mentor token_type":    func(c *MenteeClaims) { c.TokenType = "mentor_session" },
		"no session version":   func(c *MenteeClaims) { c.SessionVersion = 0 },
		"no subject":           func(c *MenteeClaims) { c.Subject = "" },
		"no audience":          func(c *MenteeClaims) { c.Audience = nil },
		"wrong audience":       func(c *MenteeClaims) { c.Audience = gojwt.ClaimStrings{"getmentor-mentor"} },
		"no expiry":            func(c *MenteeClaims) { c.ExpiresAt = nil },
		"wrong issuer":         func(c *MenteeClaims) { c.Issuer = "someone-else" },
		"expired":              func(c *MenteeClaims) { c.ExpiresAt = gojwt.NewNumericDate(now.Add(-time.Minute)) },
		"issued in the future": func(c *MenteeClaims) { c.IssuedAt = gojwt.NewNumericDate(now.Add(time.Hour)) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := valid()
			mutate(&c)
			_, err := tm.ValidateToken(signWithMenteeKey(t, tm, c))
			assert.Error(t, err)
		})
	}

	t.Run("claim-less token", func(t *testing.T) {
		_, err := tm.ValidateToken(signWithMenteeKey(t, tm, gojwt.MapClaims{"sub": "mentee-1"}))
		assert.Error(t, err)
	})

	t.Run("control: valid claims pass", func(t *testing.T) {
		_, err := tm.ValidateToken(signWithMenteeKey(t, tm, valid()))
		assert.NoError(t, err)
	})
}

func TestMenteeToken_RejectsNoneAlgorithm(t *testing.T) {
	tm := NewMenteeTokenManager(testSecret, "getmentor-api", 1)
	claims := MenteeClaims{TokenType: MenteeTokenType, SessionVersion: 1, RegisteredClaims: gojwt.RegisteredClaims{
		Subject: "mentee-1", Issuer: "getmentor-api", Audience: gojwt.ClaimStrings{MenteeAudience},
		ExpiresAt: gojwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	unsigned, err := gojwt.NewWithClaims(gojwt.SigningMethodNone, claims).SignedString(gojwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)
	_, err = tm.ValidateToken(unsigned)
	assert.Error(t, err)
}
