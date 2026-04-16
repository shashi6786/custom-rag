package config

import (
	"testing"
)

func TestLoad_DisableAuthSkipsOIDC(t *testing.T) {
	t.Setenv("DISABLE_AUTH", "true")
	t.Setenv("OIDC_ISSUER_URL", "")
	t.Setenv("OIDC_AUDIENCE", "")
	cfg, err := Load("query")
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.DisableAuth {
		t.Fatal("expected DisableAuth")
	}
}

func TestLoad_RequiresOIDCWhenAuthEnabled(t *testing.T) {
	t.Setenv("DISABLE_AUTH", "false")
	t.Setenv("OIDC_ISSUER_URL", "")
	t.Setenv("OIDC_AUDIENCE", "")
	_, err := Load("query")
	if err == nil {
		t.Fatal("expected error")
	}
}
