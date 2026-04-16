package qdrantstore

import (
	"context"
	"fmt"
	"strconv"

	"custom-rag/internal/config"
	"custom-rag/internal/domain"

	"github.com/qdrant/go-client/qdrant"
)

// SearchHit is one nearest-neighbor result.
type SearchHit struct {
	ID      string         `json:"id"`
	Score   float32        `json:"score"`
	Payload map[string]any `json:"payload,omitempty"`
}

// Corpus wraps the Qdrant client for the RAG collection.
type Corpus struct {
	client     *qdrant.Client
	collection string
	vectorSize uint64
}

// NewCorpus connects to Qdrant using cfg host/port/API key/TLS.
func NewCorpus(ctx context.Context, cfg config.Config) (*Corpus, error) {
	qcfg := &qdrant.Config{
		Host:                   cfg.QdrantHost,
		Port:                   cfg.QdrantPort,
		APIKey:                 cfg.QdrantAPIKey,
		UseTLS:                 cfg.QdrantUseTLS,
		SkipCompatibilityCheck: true,
		PoolSize:               1,
	}
	if cfg.QdrantTLSConfig != nil {
		qcfg.TLSConfig = cfg.QdrantTLSConfig
	}
	client, err := qdrant.NewClient(qcfg)
	if err != nil {
		return nil, err
	}
	return &Corpus{
		client:     client,
		collection: cfg.QdrantCollection,
		vectorSize: uint64(cfg.QdrantVectorSize),
	}, nil
}

// Close releases gRPC connections.
func (c *Corpus) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

// Health checks Qdrant availability.
func (c *Corpus) Health(ctx context.Context) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("nil corpus")
	}
	_, err := c.client.HealthCheck(ctx)
	return err
}

// EnsureCollection creates the collection if missing, or verifies vector size.
func (c *Corpus) EnsureCollection(ctx context.Context) error {
	exists, err := c.client.CollectionExists(ctx, c.collection)
	if err != nil {
		return fmt.Errorf("collection exists: %w", err)
	}
	if !exists {
		return c.client.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: c.collection,
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size:     c.vectorSize,
				Distance: qdrant.Distance_Cosine,
			}),
		})
	}
	info, err := c.client.GetCollectionInfo(ctx, c.collection)
	if err != nil {
		return fmt.Errorf("get collection info: %w", err)
	}
	got, err := vectorSizeFromInfo(info)
	if err != nil {
		return err
	}
	if got != c.vectorSize {
		return fmt.Errorf("collection %q has vector size %d, want %d", c.collection, got, c.vectorSize)
	}
	return nil
}

func vectorSizeFromInfo(info *qdrant.CollectionInfo) (uint64, error) {
	params := info.GetConfig().GetParams().GetVectorsConfig()
	if params == nil {
		return 0, fmt.Errorf("collection has no vectors config")
	}
	if p := params.GetParams(); p != nil {
		return p.GetSize(), nil
	}
	return 0, fmt.Errorf("only single-vector collections are supported in v1")
}

// Upsert inserts or replaces points in the corpus collection.
func (c *Corpus) Upsert(ctx context.Context, points []domain.Point) error {
	if len(points) == 0 {
		return nil
	}
	structs := make([]*qdrant.PointStruct, 0, len(points))
	for _, p := range points {
		if len(p.Vector) != int(c.vectorSize) {
			return fmt.Errorf("point %q: vector len %d, want %d", p.ID, len(p.Vector), c.vectorSize)
		}
		payload, err := qdrant.TryValueMap(p.Payload)
		if err != nil {
			return fmt.Errorf("point %q payload: %w", p.ID, err)
		}
		structs = append(structs, &qdrant.PointStruct{
			Id:      qdrant.NewID(p.ID),
			Vectors: qdrant.NewVectors(p.Vector...),
			Payload: payload,
		})
	}
	wait := true
	_, err := c.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: c.collection,
		Wait:           &wait,
		Points:         structs,
	})
	return err
}

// Search returns nearest neighbors by dense vector (cosine per collection config).
func (c *Corpus) Search(ctx context.Context, vector []float32, limit uint64) ([]SearchHit, error) {
	if len(vector) != int(c.vectorSize) {
		return nil, fmt.Errorf("vector len %d, want %d", len(vector), c.vectorSize)
	}
	if limit == 0 {
		limit = 10
	}
	res, err := c.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: c.collection,
		Query:          qdrant.NewQuery(vector...),
		Limit:          qdrant.PtrOf(limit),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}
	out := make([]SearchHit, 0, len(res))
	for _, sp := range res {
		out = append(out, SearchHit{
			ID:      pointIDString(sp.GetId()),
			Score:   sp.GetScore(),
			Payload: payloadToMap(sp.GetPayload()),
		})
	}
	return out, nil
}

func pointIDString(id *qdrant.PointId) string {
	if id == nil {
		return ""
	}
	if u := id.GetUuid(); u != "" {
		return u
	}
	return strconv.FormatUint(id.GetNum(), 10)
}
