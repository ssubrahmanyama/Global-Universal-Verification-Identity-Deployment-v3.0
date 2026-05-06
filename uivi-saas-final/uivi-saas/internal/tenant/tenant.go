package tenant

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

// Pool manages per-tenant DB connections
// Each tenant gets their own PostgreSQL schema: uivi_<tenant_slug>
type Pool struct {
	mu      sync.RWMutex
	conns   map[string]*sql.DB
	host    string
	user    string
	pass    string
}

func NewPool(host, user, pass string) *Pool {
	return &Pool{conns: make(map[string]*sql.DB), host: host, user: user, pass: pass}
}

// DB returns (or creates) a connection to the tenant's schema
func (p *Pool) DB(tenantSlug string) (*sql.DB, error) {
	p.mu.RLock()
	if db, ok := p.conns[tenantSlug]; ok {
		p.mu.RUnlock()
		return db, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Postgres DB names cannot contain hyphens; replace with underscore
	dbName := strings.ReplaceAll(tenantSlug, "-", "_")
	dsn := fmt.Sprintf("postgres://%s:%s@%s:5432/uivi_%s?sslmode=disable", p.user, p.pass, p.host, dbName)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(3)
	db.SetConnMaxLifetime(5 * time.Minute)
	p.conns[tenantSlug] = db
	return db, nil
}

// Handler for tenant registration / management
type Handler struct{ masterDB *sql.DB }

func New(masterDB *sql.DB) *Handler { return &Handler{masterDB} }

type RegisterRequest struct {
	OrgName   string `json:"org_name"   binding:"required"`
	OrgSlug   string `json:"org_slug"   binding:"required"` // e.g. "google", "iit-bombay"
	OrgType   string `json:"org_type"   binding:"required"` // institution|hr|regulatory|government
	Country   string `json:"country"    binding:"required"`
	AdminEmail string `json:"admin_email" binding:"required,email"`
	AdminPass  string `json:"admin_pass"  binding:"required,min=8"`
	Plan      string `json:"plan"`
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// Check slug unique
	var count int
	h.masterDB.QueryRow("SELECT COUNT(*) FROM tenants WHERE slug=$1", req.OrgSlug).Scan(&count)
	if count > 0 {
		c.JSON(409, gin.H{"error": "org slug already taken"})
		return
	}
	if req.Plan == "" {
		req.Plan = "free"
	}
	_, err := h.masterDB.Exec(`
		INSERT INTO tenants (name,slug,org_type,country,plan,status,created_at)
		VALUES ($1,$2,$3,$4,$5,'pending',NOW())`,
		req.OrgName, req.OrgSlug, req.OrgType, req.Country, req.Plan)
	if err != nil {
		c.JSON(500, gin.H{"error": "registration failed"})
		return
	}
	c.JSON(201, gin.H{
		"message": "Organization registered. Admin will approve within 24h.",
		"slug":    req.OrgSlug,
		"status":  "pending",
	})
}

func (h *Handler) List(c *gin.Context) {
	rows, _ := h.masterDB.Query("SELECT id,name,slug,org_type,country,plan,status,created_at FROM tenants ORDER BY created_at DESC")
	defer rows.Close()
	var tenants []map[string]interface{}
	for rows.Next() {
		var id int; var name, slug, orgType, country, plan, status, createdAt string
		rows.Scan(&id, &name, &slug, &orgType, &country, &plan, &status, &createdAt)
		tenants = append(tenants, map[string]interface{}{
			"id": id, "name": name, "slug": slug, "org_type": orgType,
			"country": country, "plan": plan, "status": status, "created_at": createdAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"tenants": tenants})
}

func (h *Handler) Approve(c *gin.Context) {
	id := c.Param("id")
	h.masterDB.Exec("UPDATE tenants SET status='active' WHERE id=$1", id)
	c.JSON(200, gin.H{"message": "tenant approved"})
}

func (h *Handler) Create(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// Check slug unique
	var count int
	h.masterDB.QueryRow("SELECT COUNT(*) FROM tenants WHERE slug=$1", req.OrgSlug).Scan(&count)
	if count > 0 {
		c.JSON(409, gin.H{"error": "org slug already taken"})
		return
	}
	if req.Plan == "" {
		req.Plan = "free"
	}
	_, err := h.masterDB.Exec(`
		INSERT INTO tenants (name,slug,org_type,country,plan,status,created_at)
		VALUES ($1,$2,$3,$4,$5,'active',NOW())`,
		req.OrgName, req.OrgSlug, req.OrgType, req.Country, req.Plan)
	if err != nil {
		c.JSON(500, gin.H{"error": "creation failed"})
		return
	}

	// Create tenant DB
	if err := h.createTenantDB(req.OrgSlug); err != nil {
		c.JSON(500, gin.H{"error": "db creation failed: " + err.Error()})
		return
	}

	c.JSON(201, gin.H{
		"message": "Organization created successfully.",
		"slug":    req.OrgSlug,
		"status":  "active",
	})
}

func (h *Handler) createTenantDB(slug string) error {
	// Connect to postgres DB to create the tenant DB
	dsn := "postgres://uivi:uivi@db:5432/postgres?sslmode=disable" // assuming host is db
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	dbName := strings.ReplaceAll(slug, "-", "_")
	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE uivi_%s;", dbName))
	if err != nil {
		return err
	}

	// Connect to the new DB and create tables
	tenantDSN := fmt.Sprintf("postgres://uivi:uivi@db:5432/uivi_%s?sslmode=disable", dbName)
	tenantDB, err := sql.Open("postgres", tenantDSN)
	if err != nil {
		return err
	}
	defer tenantDB.Close()

	// Enable pgcrypto
	_, err = tenantDB.Exec("CREATE EXTENSION IF NOT EXISTS pgcrypto;")
	if err != nil {
		return err
	}

	// Create tables
	_, err = tenantDB.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id            VARCHAR(64)  PRIMARY KEY,
			email         VARCHAR(256) NOT NULL UNIQUE,
			full_name     VARCHAR(256) NOT NULL,
			role          VARCHAR(32)  NOT NULL,
			password_hash VARCHAR(256) NOT NULL,
			is_active     BOOLEAN      NOT NULL DEFAULT TRUE,
			created_at    TIMESTAMP    NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

		CREATE TABLE IF NOT EXISTS certificates (
			id              VARCHAR(64)  PRIMARY KEY,
			uivid           VARCHAR(32)  NOT NULL UNIQUE,
			credential_hash VARCHAR(64)  NOT NULL UNIQUE,
			holder_name     VARCHAR(256) NOT NULL,
			holder_email    VARCHAR(256),
			holder_dob      VARCHAR(32),
			degree_type     VARCHAR(64)  NOT NULL,
			degree_name     VARCHAR(256) NOT NULL,
			specialization  VARCHAR(256),
			roll_number     VARCHAR(128) NOT NULL,
			passing_year    INT          NOT NULL,
			grade           VARCHAR(64),
			issuer_id       VARCHAR(64)  NOT NULL,
			issuer_name     VARCHAR(256) NOT NULL,
			status          VARCHAR(32)  NOT NULL DEFAULT 'active',
			revoke_reason   TEXT,
			created_at      TIMESTAMP    NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS verifications (
			id                      VARCHAR(64)  PRIMARY KEY,
			uivid                   VARCHAR(32)  NOT NULL,
			candidate_name_provided VARCHAR(256) NOT NULL,
			verifier_user_id        VARCHAR(64)  NOT NULL,
			purpose                 VARCHAR(256),
			result_valid            BOOLEAN      NOT NULL,
			verified_at             TIMESTAMP    NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS audit_events (
			id          VARCHAR(64)  PRIMARY KEY,
			user_id     VARCHAR(64)  NOT NULL,
			action      VARCHAR(64)  NOT NULL,
			details     TEXT,
			created_at  TIMESTAMP    NOT NULL DEFAULT NOW()
		);
	`)
	return err
}
