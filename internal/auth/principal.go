package auth

import "slices"

const (
	ScopeQuery  = "rag:query"
	ScopeIngest = "rag:ingest"
)

// Principal is the authenticated caller after JWT verification.
type Principal struct {
	Subject string
	Scopes  []string
}

// HasScope reports whether the principal was granted the given OAuth scope.
func (p *Principal) HasScope(want string) bool {
	return slices.Contains(p.Scopes, want)
}
