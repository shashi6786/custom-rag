package domain

// Point is one vectorized chunk stored in Qdrant.
// ID must be a UUID string (Qdrant client uses UUID point ids for string IDs).
type Point struct {
	ID      string         `json:"id"`
	Vector  []float32      `json:"vector"`
	Payload map[string]any `json:"payload"`
}
