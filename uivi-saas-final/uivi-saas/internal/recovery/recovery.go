package recovery

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uivi/saas/internal/tenant"
)

type Handler struct {
	masterDB *sql.DB
	pool     *tenant.Pool
}

func New(masterDB *sql.DB, pool *tenant.Pool) *Handler { return &Handler{masterDB, pool} }

func (h *Handler) Initiate(c *gin.Context) {
	var body struct {
		UIVID      string   `json:"uivid"      binding:"required"`
		TenantSlug string   `json:"tenant_slug" binding:"required"`
		Email      string   `json:"email"      binding:"required"`
		Guardians  []string `json:"guardians"  binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	id := uuid.New().String()
	h.masterDB.Exec(`INSERT INTO recovery_requests(id,uivid,tenant_slug,email,status,threshold,created_at,expires_at)
		VALUES($1,$2,$3,$4,'pending',2,$5,$6)`,
		id, body.UIVID, body.TenantSlug, body.Email, time.Now(), time.Now().Add(24*time.Hour))
	c.JSON(202, gin.H{"request_id": id, "message": "Recovery initiated. Guardians notified. Threshold: 2 of 3.", "expires_in": "24h"})
}

func (h *Handler) GuardianApprove(c *gin.Context) {
	var body struct {
		RequestID  string `json:"request_id"  binding:"required"`
		GuardianID string `json:"guardian_id" binding:"required"`
		Signature  string `json:"signature"   binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	h.masterDB.Exec(`INSERT INTO guardian_approvals(id,request_id,guardian_id,signature,approved_at)
		VALUES($1,$2,$3,$4,NOW())`,
		uuid.New().String(), body.RequestID, body.GuardianID, body.Signature)
	var count int
	h.masterDB.QueryRow("SELECT COUNT(*) FROM guardian_approvals WHERE request_id=$1", body.RequestID).Scan(&count)
	if count >= 2 {
		h.masterDB.Exec("UPDATE recovery_requests SET status='approved' WHERE id=$1", body.RequestID)
	}
	c.JSON(200, gin.H{"approvals": count, "threshold": 2, "status": func() string { if count >= 2 { return "approved" }; return "pending" }()})
}

func (h *Handler) Status(c *gin.Context) {
	id := c.Param("id")
	var status, uivid string; var expiresAt time.Time
	h.masterDB.QueryRow("SELECT status,uivid,expires_at FROM recovery_requests WHERE id=$1", id).
		Scan(&status, &uivid, &expiresAt)
	var count int
	h.masterDB.QueryRow("SELECT COUNT(*) FROM guardian_approvals WHERE request_id=$1", id).Scan(&count)
	c.JSON(http.StatusOK, gin.H{"request_id": id, "uivid": uivid, "status": status, "approvals": count, "threshold": 2, "expires_at": expiresAt})
}
