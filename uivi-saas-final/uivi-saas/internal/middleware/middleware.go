package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func JWT(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		tok := strings.TrimPrefix(auth, "Bearer ")
		t, err := jwt.Parse(tok, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})
		if err != nil || !t.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		claims := t.Claims.(jwt.MapClaims)
		c.Set("user_id", claims["sub"])
		c.Set("email", claims["email"])
		c.Set("full_name", claims["full_name"])
		c.Set("role", claims["role"])
		c.Set("tenant_slug", claims["tenant_slug"])
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
		role, _ := c.Get("role")
		if !set[role.(string)] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}
		c.Next()
	}
}

// Helpers to extract claims
func TenantSlug(c *gin.Context) string { v, _ := c.Get("tenant_slug"); return v.(string) }
func UserID(c *gin.Context) string     { v, _ := c.Get("user_id"); return v.(string) }
func Role_(c *gin.Context) string      { v, _ := c.Get("role"); return v.(string) }
