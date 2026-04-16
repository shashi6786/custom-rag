package ingestapi

import (
	"net/http"

	"custom-rag/internal/auth"
	"custom-rag/internal/config"
	"custom-rag/internal/domain"
	"custom-rag/internal/qdrantstore"

	"github.com/gin-gonic/gin"
)

// Register mounts ingest routes on the given group (e.g. /v1).
func Register(g *gin.RouterGroup, cfg config.Config, corpus *qdrantstore.Corpus) {
	h := &handler{cfg: cfg, corpus: corpus}
	g.POST("/ingest", h.ingest)
}

type handler struct {
	cfg    config.Config
	corpus *qdrantstore.Corpus
}

type ingestRequest struct {
	Points []domain.Point `json:"points"`
}

func (h *handler) ingest(c *gin.Context) {
	if h.corpus == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "vector store not configured"})
		return
	}
	var req ingestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json: " + err.Error()})
		return
	}
	for i, p := range req.Points {
		if p.ID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "each point needs id", "index": i}})
			return
		}
		if len(p.Vector) != h.cfg.QdrantVectorSize {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": "vector length must match QDRANT_VECTOR_SIZE",
					"index": i,
					"id":    p.ID,
					"got":   len(p.Vector),
					"want":  h.cfg.QdrantVectorSize,
				},
			})
			return
		}
	}
	if err := h.corpus.Upsert(c.Request.Context(), req.Points); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	p := auth.PrincipalFromContext(c.Request.Context())
	sub := ""
	if p != nil {
		sub = p.Subject
	}
	c.JSON(http.StatusOK, gin.H{
		"subject":  sub,
		"upserted": len(req.Points),
	})
}
