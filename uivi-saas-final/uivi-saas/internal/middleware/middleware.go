package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func JWT(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := strings.TrimSpace(c.GetHeader("Authorization"))
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		tok := strings.TrimPrefix(auth, "Bearer ")
		claims := jwt.MapClaims{}
		t, err := jwt.ParseWithClaims(tok, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		if err != nil || !t.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			return
		}

		userID, ok := claims["sub"].(string)
		if !ok || userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token payload"})
			return
		}
		role, ok := claims["role"].(string)
		if !ok || role == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token role"})
			return
		}
		tenantSlug, ok := claims["tenant_slug"].(string)
		if !ok || tenantSlug == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token tenant"})
			return
		}

		c.Set("user_id", userID)
		c.Set("email", claims["email"])
		c.Set("full_name", claims["full_name"])
		c.Set("role", role)
		c.Set("tenant_slug", tenantSlug)
		c.Set("tenant_id", claims["tenant_id"])
		c.Set("tenant_name", claims["tenant_name"])
		c.Set("org_type", claims["org_type"])
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
