package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// SECURITY FIX: Use typed RegisteredClaims instead of MapClaims for better safety.
type Claims struct {
	Role       string `json:"role"`
	TenantSlug string `json:"tenant_slug"`
	jwt.RegisteredClaims
}

func JWT(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := strings.TrimSpace(c.GetHeader("Authorization"))
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		tok := strings.TrimPrefix(auth, "Bearer ")
		
		// SECURITY FIX: Parse into typed Claims struct
		claims := &Claims{}
		t, err := jwt.ParseWithClaims(tok, claims, func(t *jwt.Token) (interface{}, error) {
			// Explicitly check algorithm is HS256 (avoid algorithm confusion attacks)
			if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token: " + err.Error()})
			return
		}
		
		if !t.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			return
		}
		
		// SECURITY FIX: Explicit expiry check (RegisteredClaims validates but double-check)
		if claims.ExpiresAt != nil && time.Until(claims.ExpiresAt.Time) <= 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token expired"})
			return
		}

		userID := claims.Subject
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token payload"})
			return
		}
		
		role := claims.Role
		if role == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token role"})
			return
		}
		
		tenantSlug := claims.TenantSlug
		if tenantSlug == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token tenant"})
			return
		}

		// SECURITY ENHANCEMENT (Future): Check jti (JWT ID) in revocation/token_store table here
		// Example: if isTokenRevoked(claims.ID) { c.AbortWithStatusJSON(401, ...) }

		// Set minimized context (avoid storing PII in gin context)
		c.Set("user_id", userID)
		c.Set("role", role)
		c.Set("tenant_slug", tenantSlug)
		c.Set("token_jti", claims.ID) // For revocation checks
		c.Next()
	}
}

func Role(allowed ...string) gin.HandlerFunc {
	set := map[string]bool{}
	for _, r := range allowed {
		set[r] = true
	}
	return func(c *gin.Context) {
		roleValue, ok := c.Get("role")
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}
		role, ok := roleValue.(string)
		if !ok || role == "" || !set[role] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}
		c.Next()
	}
}

// Helpers to extract claims
func TenantSlug(c *gin.Context) string {
	if v, ok := c.Get("tenant_slug"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func UserID(c *gin.Context) string {
	if v, ok := c.Get("user_id"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func Role_(c *gin.Context) string {
	if v, ok := c.Get("role"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func TokenJTI(c *gin.Context) string {
	if v, ok := c.Get("token_jti"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
