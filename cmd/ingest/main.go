package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"

	"custom-rag/internal/auth"
	"custom-rag/internal/config"
	"custom-rag/internal/ingestapi"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load("ingest")
	if err != nil {
		log.Fatal(err)
	}
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	ctx := context.Background()
	if cfg.OIDCSkipTLSVerify {
		ctx = oidc.ClientContext(ctx, insecureHTTPClient())
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/readyz", func(c *gin.Context) { c.Status(http.StatusOK) })

	v1 := r.Group("/v1")
	if cfg.DisableAuth {
		v1.Use(auth.GinDevPrincipal())
	} else {
		v, err := auth.NewOIDCVerifier(ctx, cfg.OIDCIssuerURL, cfg.OIDCAudience)
		if err != nil {
			log.Fatal(err)
		}
		v1.Use(auth.GinMiddleware(v, auth.ScopeIngest))
	}
	ingestapi.Register(v1)

	log.Printf("ingest listening on %s", cfg.HTTPAddr)
	if err := r.Run(cfg.HTTPAddr); err != nil {
		log.Fatal(err)
	}
}

func insecureHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // dev-only, gated by OIDC_SKIP_TLS_VERIFY
		},
	}
}
