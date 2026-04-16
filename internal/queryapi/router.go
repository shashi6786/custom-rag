package queryapi

import (
	"net/http"

	"custom-rag/internal/auth"
	"custom-rag/internal/config"
	"custom-rag/internal/qdrantstore"

	"github.com/gin-gonic/gin"
)

// Register mounts RAG query routes on the given group (e.g. /v1).
func Register(g *gin.RouterGroup, cfg config.Config, corpus *qdrantstore.Corpus) {
	h := &handler{cfg: cfg, corpus: corpus}
	g.POST("/query", h.query)
}

type handler struct {
	cfg    config.Config
	corpus *qdrantstore.Corpus
}

type queryRequest struct {
	Embedding []float32 `json:"embedding"`
	Limit     uint64    `json:"limit"`
}

func (h *handler) query(c *gin.Context) {
	if h.corpus == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "vector store not configured"})
		return
	}
	var req queryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json: " + err.Error()})
		return
	}
	if len(req.Embedding) != h.cfg.QdrantVectorSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message":       "embedding length must match QDRANT_VECTOR_SIZE",
				"got":           len(req.Embedding),
				"want":          h.cfg.QdrantVectorSize,
				"embedding_model": h.cfg.OpenAIEmbeddingModel,
			},
		})
		return
	}
	if req.Limit == 0 {
		req.Limit = 10
	}
	hits, err := h.corpus.Search(c.Request.Context(), req.Embedding, req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	p := auth.PrincipalFromContext(c.Request.Context())
	sub := ""
	if p != nil {
		sub = p.Subject
	}
	c.JSON(http.StatusOK, gin.H{
		"subject": sub,
		"hits":    hits,
	})
}
