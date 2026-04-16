package auth

import "context"

type ctxKey int

const principalKey ctxKey = 1

// WithPrincipal attaches a verified principal to the context.
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

// PrincipalFromContext returns the principal or nil.
func PrincipalFromContext(ctx context.Context) *Principal {
	v := ctx.Value(principalKey)
	if v == nil {
		return nil
	}
	p, _ := v.(*Principal)
	return p
}
