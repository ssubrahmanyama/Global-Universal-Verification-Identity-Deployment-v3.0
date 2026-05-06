package cert

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uivi/saas/internal/middleware"
	"github.com/uivi/saas/internal/tenant"
)

type Handler struct {
	pool     *tenant.Pool
	masterDB *sql.DB
}

func New(p *tenant.Pool, masterDB *sql.DB) *Handler { return &Handler{p, masterDB} }

type IssueRequest struct {
	HolderName     string `json:"holder_name"    binding:"required"`
	HolderEmail    string `json:"holder_email"   binding:"required"`
	DegreeType     string `json:"degree_type"    binding:"required"`
	DegreeName     string `json:"degree_name"    binding:"required"`
	Specialization string `json:"specialization"`
	RollNumber     string `json:"roll_number"    binding:"required"`
	PassingYear    int    `json:"passing_year"   binding:"required"`
	Grade          string `json:"grade"`
	HolderDOB      string `json:"holder_dob"`
}

func (h *Handler) Issue(c *gin.Context) {
	slug := middleware.TenantSlug(c)
	issuerID := middleware.UserID(c)
	tenantName, _ := c.Get("tenant_name")

	var req IssueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	db, err := h.pool.DB(slug)
	if err != nil {
		c.JSON(500, gin.H{"error": "db error"})
		return
	}

	// Generate UIVID + credential hash
	salt := randHex(8)
	raw := fmt.Sprintf("%s|%s|%s|%d|%s|%s", slug, req.RollNumber, req.DegreeType, req.PassingYear, req.HolderName, salt)
	h256 := sha256.Sum256([]byte(raw))
	credHash := hex.EncodeToString(h256[:])
	uivid := fmt.Sprintf("UIVI-%d-%s", time.Now().Year(), base36(credHash[:8]))
	id := uuid.New().String()

	_, dbErr := db.Exec(`
		INSERT INTO certificates
		(id, uivid, credential_hash, holder_name, holder_email, holder_dob,
		 degree_type, degree_name, specialization, roll_number, passing_year,
		 grade, issuer_id, issuer_name, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,'active',NOW())`,
		id, uivid, credHash, req.HolderName, req.HolderEmail, req.HolderDOB,
		req.DegreeType, req.DegreeName, req.Specialization, req.RollNumber,
		req.PassingYear, req.Grade, issuerID, tenantName)
	if dbErr != nil {
		c.JSON(500, gin.H{"error": "issue failed: " + dbErr.Error()})
		return
	}

	// Global index for cross-tenant retrieval and audits
	_, indexErr := h.masterDB.Exec(`
		INSERT INTO certificate_index
		(uivid, tenant_slug, issuer_name, holder_name, holder_email, holder_dob,
		 degree_type, degree_name, specialization, roll_number, passing_year,
		 grade, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NOW())`,
		uivid, slug, tenantName, req.HolderName, req.HolderEmail, req.HolderDOB,
		req.DegreeType, req.DegreeName, req.Specialization, req.RollNumber,
		req.PassingYear, req.Grade, "active")
	if indexErr != nil {
		c.JSON(500, gin.H{"error": "indexing failed: " + indexErr.Error()})
		return
	}

	// Audit
	db.Exec(`INSERT INTO audit_events(id,user_id,action,details,created_at)
		VALUES($1,$2,'CERTIFICATE_ISSUED',$3,NOW())`,
		uuid.New().String(), issuerID,
		fmt.Sprintf(`{"uivid":"%s","holder":"%s"}`, uivid, req.HolderName))

	c.JSON(201, gin.H{
		"uivid": uivid, "credential_hash": credHash,
		"certificate_id": id, "holder_name": req.HolderName,
		"degree_type": req.DegreeType, "passing_year": req.PassingYear,
		"issued_by": tenantName, "issued_at": time.Now(),
		"message": "Certificate issued and anchored successfully.",
	})
}

func (h *Handler) List(c *gin.Context) {
	slug := middleware.TenantSlug(c)
	db, _ := h.pool.DB(slug)
	rows, _ := db.Query(`SELECT uivid,holder_name,degree_type,passing_year,status,created_at
		FROM certificates ORDER BY created_at DESC LIMIT 100`)
	defer rows.Close()
	var certs []map[string]interface{}
	for rows.Next() {
		var uivid, name, degree, status, createdAt string
		var year int
		rows.Scan(&uivid, &name, &degree, &year, &status, &createdAt)
		certs = append(certs, map[string]interface{}{
			"uivid": uivid, "holder_name": name, "degree_type": degree,
			"passing_year": year, "status": status, "created_at": createdAt,
		})
	}
	if certs == nil {
		certs = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"certificates": certs, "count": len(certs)})
}

func (h *Handler) Revoke(c *gin.Context) {
	slug := middleware.TenantSlug(c)
	id := c.Param("id")
	var body struct {
		Reason string `json:"reason"`
	}
	c.ShouldBindJSON(&body)
	db, _ := h.pool.DB(slug)
	db.Exec("UPDATE certificates SET status='revoked',revoke_reason=$1 WHERE id=$2", body.Reason, id)

	var uivid string
	db.QueryRow("SELECT uivid FROM certificates WHERE id=$1", id).Scan(&uivid)
	if uivid != "" {
		h.masterDB.Exec("UPDATE certificate_index SET status='revoked', revoke_reason=$1, revoked_at=NOW() WHERE uivid=$2", body.Reason, uivid)
	}
	c.JSON(200, gin.H{"message": "revoked"})
}

func (h *Handler) Get(c *gin.Context) {
	uivid := c.Param("uivid")
	if uivid == "" {
		c.JSON(400, gin.H{"error": "uivid required"})
		return
	}

	var cert struct {
		TenantSlug     string
		IssuerName     string
		HolderName     string
		HolderEmail    sql.NullString
		HolderDOB      sql.NullString
		DegreeType     string
		DegreeName     string
		Specialization sql.NullString
		RollNumber     string
		PassingYear    int
		Grade          sql.NullString
		Status         string
		RevokeReason   sql.NullString
		CreatedAt      time.Time
		RevokedAt      sql.NullTime
	}
	err := h.masterDB.QueryRow(`
		SELECT tenant_slug, issuer_name, holder_name, holder_email, holder_dob,
		 degree_type, degree_name, specialization, roll_number, passing_year,
		 grade, status, revoke_reason, created_at, revoked_at
		FROM certificate_index WHERE uivid=$1`, uivid).
		Scan(&cert.TenantSlug, &cert.IssuerName, &cert.HolderName, &cert.HolderEmail,
			&cert.HolderDOB, &cert.DegreeType, &cert.DegreeName, &cert.Specialization,
			&cert.RollNumber, &cert.PassingYear, &cert.Grade, &cert.Status,
			&cert.RevokeReason, &cert.CreatedAt, &cert.RevokedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "certificate not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"uivid":          uivid,
		"tenant_slug":    cert.TenantSlug,
		"issuer_name":    cert.IssuerName,
		"holder_name":    cert.HolderName,
		"holder_email":   cert.HolderEmail.String,
		"holder_dob":     cert.HolderDOB.String,
		"degree_type":    cert.DegreeType,
		"degree_name":    cert.DegreeName,
		"specialization": cert.Specialization.String,
		"roll_number":    cert.RollNumber,
		"passing_year":   cert.PassingYear,
		"grade":          cert.Grade.String,
		"status":         cert.Status,
		"revoke_reason":  cert.RevokeReason.String,
		"created_at":     cert.CreatedAt,
		"revoked_at":     cert.RevokedAt.Time,
	})
}

func (h *Handler) ListAll(c *gin.Context) {
	rows, err := h.masterDB.Query(`
		SELECT uivid, tenant_slug, issuer_name, holder_name, degree_type,
		 degree_name, passing_year, status, created_at, revoke_reason
		FROM certificate_index ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		c.JSON(500, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	var list []map[string]interface{}
	for rows.Next() {
		var uivid, tenantSlug, issuerName, holderName, degreeType, degreeName, status, revokeReason sql.NullString
		var passingYear int
		var createdAt time.Time
		rows.Scan(&uivid, &tenantSlug, &issuerName, &holderName, &degreeType, &degreeName, &passingYear, &status, &createdAt, &revokeReason)
		list = append(list, map[string]interface{}{
			"uivid":         uivid.String,
			"tenant_slug":   tenantSlug.String,
			"issuer_name":   issuerName.String,
			"holder_name":   holderName.String,
			"degree_type":   degreeType.String,
			"degree_name":   degreeName.String,
			"passing_year":  passingYear,
			"status":        status.String,
			"created_at":    createdAt,
			"revoke_reason": revokeReason.String,
		})
	}
	if list == nil {
		list = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"certificates": list, "count": len(list)})
}

func (h *Handler) Dashboard(c *gin.Context) {
	slug := middleware.TenantSlug(c)
	db, _ := h.pool.DB(slug)
	var total, active, revoked int
	db.QueryRow("SELECT COUNT(*) FROM certificates").Scan(&total)
	db.QueryRow("SELECT COUNT(*) FROM certificates WHERE status='active'").Scan(&active)
	db.QueryRow("SELECT COUNT(*) FROM certificates WHERE status='revoked'").Scan(&revoked)
	c.JSON(200, gin.H{"total_issued": total, "active": active, "revoked": revoked})
}

func base36(s string) string {
	const chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	out := make([]byte, len(s))
	for i, b := range []byte(s) {
		out[i] = chars[int(b)%36]
	}
	return string(out)
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
