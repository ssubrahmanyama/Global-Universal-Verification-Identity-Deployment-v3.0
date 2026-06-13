package main

import (
	"database/sql"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"

	"github.com/uivi/saas/internal/audit"
	"github.com/uivi/saas/internal/auth"
	"github.com/uivi/saas/internal/cert"
	"github.com/uivi/saas/internal/fraud"
	"github.com/uivi/saas/internal/middleware"
	"github.com/uivi/saas/internal/recovery"
	"github.com/uivi/saas/internal/regulatory"
	"github.com/uivi/saas/internal/tenant"
	"github.com/uivi/saas/internal/verify"
)

func main() {
	masterDB := mustDB(env("MASTER_DB_URL", "postgres://uivi:uivi@db:5432/uivi_master?sslmode=disable"))
	pool := tenant.NewPool(env("TENANT_DB_HOST", "db"), env("TENANT_DB_USER", "uivi"), env("TENANT_DB_PASS", "uivi"))
	
	// SECURITY FIX: Enforce JWT_SECRET presence and minimum length (>=32 chars)
	secret := env("JWT_SECRET", "")
	if secret == "" {
		log.Fatalf("FATAL: JWT_SECRET environment variable must be set for production security")
	}
	if len(secret) < 32 {
		log.Fatalf("FATAL: JWT_SECRET must be at least 32 characters long for HS256; current length=%d", len(secret))
	}

	authH := auth.New(masterDB, pool, secret)
	certH := cert.New(pool, masterDB)
	verH := verify.New(pool)
	audH := audit.New(pool)
	frH := fraud.New(pool, masterDB)
	regH := regulatory.New(masterDB, pool)
	recH := recovery.New(masterDB, pool)
	tenH := tenant.New(masterDB)

	r := gin.Default()
	
	// SECURITY FIX: Restrict CORS to configured origins instead of wildcard (*)
	allowedOrigins := env("ALLOWED_ORIGINS", "http://localhost:8080")
	corsConfig := cors.DefaultConfig()
	if allowedOrigins == "*" || allowedOrigins == "" {
		// Default: restrict to localhost for development
		corsConfig.AllowOrigins = []string{"http://localhost:8080"}
		log.Printf("WARNING: CORS restricted to localhost. Set ALLOWED_ORIGINS for production (comma-separated).")
	} else {
		corsConfig.AllowOrigins = strings.Split(allowedOrigins, ",")
	}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Authorization", "Content-Type"}
	corsConfig.AllowCredentials = true
	corsConfig.MaxAge = 12 * time.Hour

	r.Use(cors.New(corsConfig))

	// Frontend
	r.Static("/static", "./frontend")
	r.StaticFile("/", "./frontend/index.html")
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	// Public
	r.POST("/api/v1/auth/login", authH.Login)
	r.POST("/api/v1/auth/register/user", authH.RegisterUser)
	r.POST("/api/v1/tenant/register", tenH.Register)

	// Recovery (semi-public — email-based)
	r.POST("/api/v1/recovery/initiate", recH.Initiate)
	r.POST("/api/v1/recovery/guardian/approve", recH.GuardianApprove)
	r.GET("/api/v1/recovery/status/:id", recH.Status)

	// JWT-protected
	api := r.Group("/api/v1", middleware.JWT(secret))

	// Institution
	api.POST("/institution/certificate/issue", middleware.Role("institution", "admin"), certH.Issue)
	api.GET("/institution/certificates", middleware.Role("institution", "admin"), certH.List)
	api.POST("/institution/certificate/:id/revoke", middleware.Role("institution", "admin"), certH.Revoke)
	api.GET("/institution/dashboard", middleware.Role("institution", "admin"), certH.Dashboard)

	// HR
	api.POST("/hr/verify", middleware.Role("hr", "admin"), verH.Verify)
	api.POST("/hr/verify/batch", middleware.Role("hr", "admin"), verH.BatchVerify)
	api.GET("/hr/verifications", middleware.Role("hr", "admin"), verH.History)
	api.GET("/hr/dashboard", middleware.Role("hr", "admin"), verH.Dashboard)
	api.GET("/hr/certificate/:uivid", middleware.Role("hr", "admin"), certH.Get)

	// User (all roles can access own profile/audit)
	api.GET("/user/audit-trail", audH.MyTrail)
	api.GET("/user/profile", audH.MyProfile)
	api.POST("/user/consent", audH.SetConsent)

	// Regulatory
	api.GET("/regulatory/dashboard", middleware.Role("regulatory", "admin"), regH.Dashboard)
	api.GET("/regulatory/institutions", middleware.Role("regulatory", "admin"), regH.ListInstitutions)
	api.GET("/regulatory/stats", middleware.Role("regulatory", "admin"), regH.Stats)
	api.GET("/regulatory/compliance", middleware.Role("regulatory", "admin"), regH.Compliance)
	api.GET("/regulatory/audit-logs", middleware.Role("regulatory", "admin"), regH.AuditLogs)
	api.GET("/regulatory/certificates", middleware.Role("regulatory", "admin"), certH.ListAll)

	// Fraud
	api.GET("/fraud-monitor/dashboard", middleware.Role("fraud_monitor", "regulatory", "admin"), frH.Dashboard)
	api.GET("/fraud-monitor/alerts", middleware.Role("fraud_monitor", "regulatory", "admin"), frH.Alerts)
	api.GET("/fraud-monitor/certificate/:uivid", middleware.Role("fraud_monitor", "regulatory", "admin"), certH.Get)
	api.POST("/fraud-monitor/alerts/:id/report", middleware.Role("fraud_monitor", "regulatory", "admin"), frH.Report)
	api.GET("/fraud-monitor/graph", middleware.Role("fraud_monitor", "regulatory", "admin"), frH.Graph)

	// Admin
	api.GET("/admin/tenants", middleware.Role("admin"), tenH.List)
	api.POST("/admin/tenants", middleware.Role("admin"), tenH.Create)
	api.POST("/admin/tenants/:id/approve", middleware.Role("admin"), tenH.Approve)

	port := env("PORT", "8080")
	log.Printf("UIVI SaaS → http://localhost:%s", port)
	log.Printf("NOTE: Run behind a TLS termination proxy (nginx/load-balancer) in production.")
	r.Run(":" + port)
}

func mustDB(dsn string) *sql.DB {
	for i := 0; i < 15; i++ {
		db, err := sql.Open("postgres", dsn)
		if err == nil {
			if db.Ping() == nil {
				db.SetMaxOpenConns(25)
				return db
			}
		}
		log.Printf("DB not ready (%d/15)...", i+1)
		time.Sleep(3 * time.Second)
	}
	log.Fatal("cannot connect to master DB")
	return nil
}

func env(k, v string) string {
	if s := os.Getenv(k); s != "" {
		return s
	}
	return v
}
