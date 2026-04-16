package auth

import "github.com/gin-gonic/gin"

// GinDevPrincipal attaches a synthetic principal when DISABLE_AUTH is enabled.
func GinDevPrincipal() gin.HandlerFunc {
	return func(c *gin.Context) {
		p := &Principal{
			Subject: "dev",
			Scopes:  []string{ScopeQuery, ScopeIngest},
		}
		ctx := WithPrincipal(c.Request.Context(), p)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
