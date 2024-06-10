package api

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"

	"github.com/arryved/app-ctrl/api/config"
)

type GoogleIDTokenClaims struct {
	AtHash        string `json:"at_hash"`
	Aud           string `json:"aud"`
	Azp           string `json:"azp"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Exp           int64  `json:"exp"`
	FamilyName    string `json:"family_name"`
	GivenName     string `json:"given_name"`
	Hd            string `json:"hd"`
	Iat           int64  `json:"iat"`
	Iss           string `json:"iss"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	Sub           string `json:"sub"`
	jwt.RegisteredClaims
}

func generateSecretKey() (*rsa.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Println("Error generating RSA key:", err)
		return nil, err
	}
	return privateKey, nil
}

func generateFakeIDToken() (string, error) {
	secretKey, err := generateSecretKey()
	if err != nil {
		panic("could not generate secret key")
	}
	// Create the claims
	now := time.Now()
	claims := GoogleIDTokenClaims{
		AtHash:        "SB0fQxn-fr8FLN4JETCznQ",
		Aud:           "123-abc.apps.googleusercontent.com",
		Azp:           "123-abc.apps.googleusercontent.com",
		Email:         "mockuser@example.com",
		EmailVerified: true,
		Exp:           now.Add(time.Hour * 24).Unix(),
		FamilyName:    "Mock",
		GivenName:     "User",
		Hd:            "example.com",
		Iat:           now.Unix(),
		Iss:           "https://accounts.google.com",
		Name:          "Mock User",
		Picture:       "https://example.com/picture.jpg",
		Sub:           "1234567890",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://accounts.google.com",
			Subject:   "1234567890",
			Audience:  []string{"123-abc.apps.googleusercontent.com"},
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour * 24)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        "mockID",
		},
	}

	// Create the token using the claims
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	// Sign the token with the secret key
	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func TestAuthenticated(t *testing.T) {
	assert := assert.New(t)
	cfg := config.Load("../config/mock-config.yml")
	cfg.AuthnEnabled = true
	fake_token, err := generateFakeIDToken()
	assert.NoError(err)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", fake_token))

	result := authenticated(cfg, req)

	assert.False(result)
}
