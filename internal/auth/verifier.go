package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

// TokenVerifier validates raw Bearer JWT access tokens from the OIDC issuer.
type TokenVerifier interface {
	Verify(ctx context.Context, rawAccessToken string) (*Principal, error)
}

// OIDCVerifier uses go-oidc to validate Keycloak-issued JWT access tokens.
type OIDCVerifier struct {
	verifier *oidc.IDTokenVerifier
}

// NewOIDCVerifier builds a verifier for the given issuer URL and expected audience
// (Keycloak client id or configured audience string).
func NewOIDCVerifier(ctx context.Context, issuerURL, audience string) (*OIDCVerifier, error) {
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc provider: %w", err)
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: audience})
	return &OIDCVerifier{verifier: verifier}, nil
}

type accessTokenClaims struct {
	Scope       string   `json:"scope"`
	ScopeArray  []string `json:"scp"`
	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

// Verify checks signature, issuer, expiry, audience, and extracts subject + scopes.
func (v *OIDCVerifier) Verify(ctx context.Context, rawAccessToken string) (*Principal, error) {
	idt, err := v.verifier.Verify(ctx, rawAccessToken)
	if err != nil {
		return nil, err
	}
	var c accessTokenClaims
	if err := idt.Claims(&c); err != nil {
		return nil, fmt.Errorf("claims: %w", err)
	}
	scopes := parseScopes(c)
	return &Principal{Subject: idt.Subject, Scopes: scopes}, nil
}

func parseScopes(c accessTokenClaims) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	for _, p := range strings.Fields(c.Scope) {
		add(p)
	}
	for _, s := range c.ScopeArray {
		add(s)
	}
	for _, r := range c.RealmAccess.Roles {
		add(r)
	}
	return out
}
