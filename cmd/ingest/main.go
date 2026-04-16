package main

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"time"

	"custom-rag/internal/auth"
	"custom-rag/internal/config"
	"custom-rag/internal/ingestapi"
	"custom-rag/internal/qdrantstore"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load("ingest")
	if err != nil {
		log.Fatal(err)
	}
	if cfg.QdrantUseTLS && cfg.QdrantSkipTLSVerify {
		cfg.QdrantTLSConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // gated by QDRANT_SKIP_TLS_VERIFY
	}
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	corpus, err := qdrantstore.NewCorpus(context.Background(), cfg)
	if err != nil {
		log.Fatalf("qdrant: %v", err)
	}
	defer corpus.Close()

	if err := corpus.EnsureCollection(context.Background()); err != nil {
		log.Fatalf("ensure collection: %v", err)
	}

	oidcCtx := context.Background()
	if cfg.OIDCSkipTLSVerify {
		oidcCtx = oidc.ClientContext(oidcCtx, insecureHTTPClient())
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
		})
	})
	r.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := corpus.Health(ctx); err != nil {
			c.String(http.StatusServiceUnavailable, "qdrant: %v", err)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"qdrant": "healthy",
		})
	})

	v1 := r.Group("/v1")
	if cfg.DisableAuth {
		v1.Use(auth.GinDevPrincipal())
	} else {
		v, err := auth.NewOIDCVerifier(oidcCtx, cfg.OIDCIssuerURL, cfg.OIDCAudience)
		if err != nil {
			log.Fatal(err)
		}
		v1.Use(auth.GinMiddleware(v, auth.ScopeIngest))
	}
	ingestapi.Register(v1, cfg, corpus)

	log.Printf("ingest listening on %s", cfg.HTTPAddr)
	if err := r.Run(cfg.HTTPAddr); err != nil {
		log.Fatal(err)
	}
}

func insecureHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // gated by OIDC_SKIP_TLS_VERIFY
		},
	}
}
