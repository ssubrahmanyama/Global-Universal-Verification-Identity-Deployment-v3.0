package auth

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Pool interface {
	DB(tenantSlug string) (*sql.DB, error)
}

type Handler struct {
	masterDB *sql.DB
	pool     Pool
	secret   string
}

func New(masterDB *sql.DB, pool Pool, secret string) *Handler {
	return &Handler{masterDB, pool, secret}
}

type LoginRequest struct {
	Email      string `json:"email"       binding:"required,email"`
	Password   string `json:"password"    binding:"required"`
	TenantSlug string `json:"tenant_slug" binding:"required"`
}

type RegisterUserRequest struct {
	TenantSlug string `json:"tenant_slug" binding:"required"`
	Email      string `json:"email"       binding:"required,email"`
	Password   string `json:"password"    binding:"required,min=8"`
	FullName   string `json:"full_name"   binding:"required"`
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Validate tenant exists and is active
	var tenantID int
	var tenantName, orgType, status string
	err := h.masterDB.QueryRow(
		"SELECT id,name,org_type,status FROM tenants WHERE slug=$1", req.TenantSlug).
		Scan(&tenantID, &tenantName, &orgType, &status)
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid tenant"})
		return
	}
	if status != "active" {
		c.JSON(403, gin.H{"error": "tenant not yet approved"})
		return
	}

	// Get user from tenant-specific DB
	db, err := h.pool.DB(req.TenantSlug)
	if err != nil {
		c.JSON(500, gin.H{"error": "tenant db unavailable"})
		return
	}

	var userID, role, fullName, passwordHash string
	err = db.QueryRow(
		"SELECT id,role,full_name,password_hash FROM users WHERE email=$1 AND is_active=true", req.Email).
		Scan(&userID, &role, &fullName, &passwordHash)
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)) != nil {
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}

	// SECURITY FIX: Use typed RegisteredClaims with jti for token revocation support
	jti := uuid.New().String()
	claims := &struct {
		Role       string `json:"role"`
		TenantSlug string `json:"tenant_slug"`
		jwt.RegisteredClaims
	}{
		Role:       role,
		TenantSlug: req.TenantSlug,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(8 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        jti,
			Issuer:    "uivi-saas",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(h.secret))
	if err != nil {
		log.Printf("error: unable to sign token: %v", err)
		c.JSON(500, gin.H{"error": "unable to sign token"})
		return
	}

	// SECURITY ENHANCEMENT: Persist token jti in master DB for revocation checks
	// In production, use Redis with TTL expiry instead of DB for performance
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := h.masterDB.ExecContext(ctx,
		`INSERT INTO token_store (jti, user_id, tenant_slug, expires_at, created_at) 
		 VALUES ($1,$2,$3,$4,NOW())
		 ON CONFLICT (jti) DO NOTHING`,
		jti, userID, req.TenantSlug, time.Now().Add(8*time.Hour)); err != nil {
		log.Printf("warning: failed to store token jti: %v", err)
		// Non-fatal; log but continue (token still valid even if jti not stored)
	}

	// Log login event in tenant DB with timeout
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	if _, err := db.ExecContext(ctx2,
		"INSERT INTO audit_events(id,user_id,action,details,created_at) VALUES($1,$2,'LOGIN','{}',NOW())",
		uuid.New().String(), userID); err != nil {
		log.Printf("warning: audit insert failed: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"token":       tokenStr,
		"user_id":     userID,
		"email":       req.Email,
		"role":        role,
		"tenant_slug": req.TenantSlug,
		"expires_in":  "8h",
	})
}

func (h *Handler) RegisterUser(c *gin.Context) {
	var req RegisterUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Ensure the provided tenant exists and is active before creating users.
	var tenantID int
	var tenantName, orgType, status string
	err := h.masterDB.QueryRow(
		"SELECT id,name,org_type,status FROM tenants WHERE slug=$1", req.TenantSlug).
		Scan(&tenantID, &tenantName, &orgType, &status)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid tenant"})
		return
	}
	if status != "active" {
		c.JSON(403, gin.H{"error": "tenant not yet approved"})
		return
	}

	db, err := h.pool.DB(req.TenantSlug)
	if err != nil {
		c.JSON(500, gin.H{"error": "tenant db unavailable"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(500, gin.H{"error": "unable to hash password"})
		return
	}

	id := uuid.New().String()
	role := "user"
	
	// Use context timeout for user insertion
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, dbErr := db.ExecContext(ctx, `
		INSERT INTO users(id,email,full_name,role,password_hash,is_active,created_at)
		VALUES($1,$2,$3,$4,$5,true,NOW())`,
		id, req.Email, req.FullName, role, string(hash))
	if dbErr != nil {
		c.JSON(409, gin.H{"error": "email already registered"})
		return
	}
	c.JSON(201, gin.H{"user_id": id, "email": req.Email, "role": role, "tenant": req.TenantSlug})
}
