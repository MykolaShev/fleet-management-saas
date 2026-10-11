package auth

import (
	"encoding/json"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleConfig holds the Google Cloud OAuth2 client details. See README.md
// for how to obtain these — a free Google Cloud project, no billing
// account or special access required.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// NewOAuth2Config builds the golang.org/x/oauth2 config for Google. The
// x/oauth2/google subpackage already ships Google's endpoint URLs, so
// there's nothing to hand-construct here (unlike Microsoft, which needed
// its own base-URL formatting).
func NewOAuth2Config(cfg GoogleConfig) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Scopes:       []string{"openid", "profile", "email"},
		Endpoint:     google.Endpoint,
	}
}

// GoogleClaims is the subset of ID token claims we actually use. Unlike
// Microsoft's token, there's no organization/tenant claim here — a plain
// Google account doesn't belong to one. See AuthService.HandleGoogleLogin
// for how that's handled (a new tenant is created per new user instead of
// reused from a claim).
type GoogleClaims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
}

// ExtractIDToken pulls the ID token out of a token response. Per the OIDC
// spec, it rides along as an extra field, not inside the access token.
func ExtractIDToken(tok *oauth2.Token) (string, error) {
	raw, ok := tok.Extra("id_token").(string)
	if !ok || raw == "" {
		return "", fmt.Errorf("token response did not include an id_token")
	}
	return raw, nil
}

// ParseIDToken extracts claims from the ID token Google returned.
//
// Deliberate simplification for this project: this does NOT verify the ID
// token's signature against Google's published JWKS keys. That
// verification is normally required because an ID token might arrive from
// anywhere — but here it comes directly from Google's token endpoint, over
// TLS, in response to a request authenticated with our own client secret.
// Forging it would require compromising that channel or our secret, not
// just handing us a crafted token. A production system would still add
// JWKS verification as defense in depth (e.g. via
// github.com/coreos/go-oidc); skipping it here keeps the dependency
// surface and failure modes smaller for a project already juggling
// several other moving parts.
func ParseIDToken(rawIDToken string) (*GoogleClaims, error) {
	parser := jwt.NewParser()
	token, _, err := parser.ParseUnverified(rawIDToken, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("parse id_token: %w", err)
	}

	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("parse id_token: unexpected claims type")
	}

	raw, err := json.Marshal(mapClaims)
	if err != nil {
		return nil, fmt.Errorf("parse id_token: %w", err)
	}

	var claims GoogleClaims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil, fmt.Errorf("parse id_token: %w", err)
	}

	if claims.Subject == "" {
		return nil, fmt.Errorf("parse id_token: missing sub claim")
	}

	return &claims, nil
}