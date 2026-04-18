package auth

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/uivi/saas/internal/tenant"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	masterDB *sql.DB
	pool     *tenant.Pool
	secret   string
}

func New(masterDB *sql.DB, pool *tenant.Pool, secret string) *Handler {
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
	Role       string `json:"role"        binding:"required"` // institution|hr|user|regulatory|fraud_monitor|admin
	FullName   string `json:"full_name"   binding:"required"`
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Validate tenant exists and is active
	var tenantID int; var tenantName, orgType, status string
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

	// Mint JWT with tenant + role claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":         userID,
		"email":       req.Email,
		"full_name":   fullName,
		"role":        role,
		"tenant_slug": req.TenantSlug,
		"tenant_id":   tenantID,
		"tenant_name": tenantName,
		"org_type":    orgType,
		"exp":         time.Now().Add(8 * time.Hour).Unix(),
		"iat":         time.Now().Unix(),
	})
	tokenStr, _ := token.SignedString([]byte(h.secret))

	// Log login event in tenant DB
	db.Exec("INSERT INTO audit_events(id,user_id,action,details,created_at) VALUES($1,$2,'LOGIN','{}',NOW())",
		uuid.New().String(), userID)

	c.JSON(http.StatusOK, gin.H{
		"token":       tokenStr,
		"user_id":     userID,
		"email":       req.Email,
		"full_name":   fullName,
		"role":        role,
		"tenant_slug": req.TenantSlug,
		"tenant_name": tenantName,
		"org_type":    orgType,
		"expires_in":  "8h",
	})
}

func (h *Handler) RegisterUser(c *gin.Context) {
	var req RegisterUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	db, err := h.pool.DB(req.TenantSlug)
	if err != nil {
		c.JSON(500, gin.H{"error": "tenant db unavailable"})
		return
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	id := uuid.New().String()
	_, dbErr := db.Exec(`
		INSERT INTO users(id,email,full_name,role,password_hash,is_active,created_at)
		VALUES($1,$2,$3,$4,$5,true,NOW())`,
		id, req.Email, req.FullName, req.Role, string(hash))
	if dbErr != nil {
		c.JSON(409, gin.H{"error": "email already registered"})
		return
	}
	c.JSON(201, gin.H{"user_id": id, "email": req.Email, "role": req.Role, "tenant": req.TenantSlug})
}
