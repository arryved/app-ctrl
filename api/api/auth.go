package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc"

	"github.com/arryved/app-ctrl/api/config"
)

func authenticated(cfg *config.Config, r *http.Request) bool {
	if !cfg.AuthnEnabled {
		log.Warnf("Authentication disabled, no login is required!")
		return true
	}
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		log.Warnf("Authorization header missing")
		return false
	}

	authValue := strings.Split(authHeader, "Bearer")
	if len(authValue) < 2 {
		log.Warnf("Authorization header value not in correct format for bearer token")
		return false
	}

	idToken := strings.TrimSpace(authValue[1])
	claims, err := parseGoogleIDToken(idToken)
	if err != nil {
		log.Warnf("Authorization header token value could not be parsed err=%s", err.Error())
		return false
	}

	log.Infof("claims=%v", claims)
	return true
}

func parseGoogleIDToken(token string) (map[string]interface{}, error) {
	ctx := context.Background()

	// Set up the OIDC provider using Google's endpoints
	provider, err := oidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return nil, fmt.Errorf("Failed to get provider: %s", err.Error())
	}

	decoded := decodeIDToken(token)

	// Configure an ID token verifier using the provider and the client ID
	verifier := provider.Verifier(&oidc.Config{
		ClientID: decoded["aud"].(string),
	})

	// Parse and verify the ID token
	tokenObj, err := verifier.Verify(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("Failed to verify ID token: %s", err.Error())
	}

	// Extract the claims
	var claims map[string]interface{}
	if err := tokenObj.Claims(&claims); err != nil {
		return nil, fmt.Errorf("Failed to extract claims: %s", err.Error())
	}

	return claims, nil
}

func decodeIDToken(idToken string) map[string]interface{} {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		log.Warn("Invalid ID token format")
		return nil
	}

	seg := strings.TrimRight(parts[1], "=")
	payload, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		log.Warnf("Failed to decode payload err=%s", err.Error())
		return nil
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		log.Warnf("Failed to unmarshal claims err=%s", err.Error())
		return nil
	}
	return claims
}

func getClaims(r *http.Request) map[string]interface{} {
	authHeader := r.Header.Get("Authorization")
	authValue := strings.Split(authHeader, "Bearer")
	idToken := strings.TrimSpace(authValue[1])
	return decodeIDToken(idToken)
}
