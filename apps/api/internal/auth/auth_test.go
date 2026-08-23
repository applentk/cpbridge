package auth_test

import (
	"testing"
	"time"

	"github.com/cp-hub/api/internal/auth"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestTokenParsing(t *testing.T) {
	secret := []byte("test-secret-key-1234567890123456")
	user := &auth.User{
		ID:       "usr_123456",
		Username: "testuser",
		Role:     auth.RoleAdmin,
	}

	claims := auth.Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(secret)
	assert.NoError(t, err)

	parsedToken, err := jwt.ParseWithClaims(tokenStr, &auth.Claims{}, func(token *jwt.Token) (any, error) {
		return secret, nil
	})
	assert.NoError(t, err)
	parsedClaims, ok := parsedToken.Claims.(*auth.Claims)
	assert.True(t, ok)
	assert.Equal(t, "usr_123456", parsedClaims.UserID)
	assert.Equal(t, "testuser", parsedClaims.Username)
	assert.Equal(t, auth.RoleAdmin, parsedClaims.Role)
}
