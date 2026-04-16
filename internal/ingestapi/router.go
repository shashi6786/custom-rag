package ingestapi

import (
	"net/http"

	"custom-rag/internal/auth"

	"github.com/gin-gonic/gin"
)

// Register mounts ingest routes on the given group (e.g. /v1).
func Register(g *gin.RouterGroup) {
	g.POST("/ingest", ingestStub)
}

func ingestStub(c *gin.Context) {
	p := auth.PrincipalFromContext(c.Request.Context())
	sub := ""
	if p != nil {
		sub = p.Subject
	}
	c.JSON(http.StatusNotImplemented, gin.H{
		"status":  "not_implemented",
		"message": "Corpus ingest pipeline will be implemented next",
		"subject": sub,
	})
}
