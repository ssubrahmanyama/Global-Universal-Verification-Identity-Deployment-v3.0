package verify

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/uivi/saas/internal/middleware"
	"github.com/uivi/saas/internal/tenant"
)

type Handler struct{ pool *tenant.Pool }

func New(p *tenant.Pool) *Handler { return &Handler{p} }

type VerifyRequest struct {
	UIVID         string `json:"uivid"          binding:"required"`
	CandidateName string `json:"candidate_name" binding:"required"`
	Purpose       string `json:"purpose"`
}

type BatchVerifyRequest struct {
	Candidates []VerifyRequest `json:"candidates" binding:"required,min=1,max=50"`
}

func (h *Handler) Verify(c *gin.Context) {
	hrSlug := middleware.TenantSlug(c)
	hrUserID := middleware.UserID(c)
	hrName, _ := c.Get("tenant_name")

	var req VerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	result := h.performVerify(req.UIVID, req.CandidateName, hrSlug, fmt.Sprintf("%v", hrName), hrUserID, req.Purpose)
	c.JSON(http.StatusOK, result)
}

func (h *Handler) BatchVerify(c *gin.Context) {
	hrSlug := middleware.TenantSlug(c)
	hrUserID := middleware.UserID(c)
	hrName, _ := c.Get("tenant_name")

	var req BatchVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	results := make([]map[string]interface{}, 0)
	valid, invalid := 0, 0
	for _, cand := range req.Candidates {
		r := h.performVerify(cand.UIVID, cand.CandidateName, hrSlug, fmt.Sprintf("%v", hrName), hrUserID, cand.Purpose)
		results = append(results, r)
		if r["credential_valid"] == true {
			valid++
		} else {
			invalid++
		}
	}
	c.JSON(200, gin.H{
		"batch_id":  uuid.New().String(),
		"processed": len(results),
		"summary":   gin.H{"valid": valid, "invalid": invalid},
		"results":   results,
	})
}

// performVerify does a cross-tenant lookup: searches ALL tenant DBs for the UIVID
func (h *Handler) performVerify(uivid, candidateName, hrSlug, hrName, hrUserID, purpose string) map[string]interface{} {
	requestID := uuid.New().String()
	now := time.Now()

	// Search across all known tenant DBs for this UIVID
	var cert struct {
		holderName, degreeType, degreeName, issuerName, status string
		passingYear                                              int
	}
	found := false

	// We scan all pools — in production, maintain an index of uivid→tenant_slug in master DB
	// For now, try a broadcast search approach via master DB index
	hrDB, err := h.pool.DB(hrSlug)
	if err == nil {
		// Log this verification in HR's own DB
		defer func() {
			hrDB.Exec(`INSERT INTO verifications
				(id,uivid,candidate_name_provided,verifier_user_id,purpose,result_valid,verified_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7)`,
				requestID, uivid, candidateName, hrUserID, purpose, found, now)
		}()
	}

	// Try to find from UIVID prefix — extract issuer slug from pattern
	// UIVID format: UIVI-YEAR-HASH — we store a global index in master
	// This simulates lookup; in full deployment use master index table
	slugsToTry := h.getSlugCandidates(uivid)
	for _, slug := range slugsToTry {
		db, dbErr := h.pool.DB(slug)
		if dbErr != nil {
			continue
		}
		err := db.QueryRow(`
			SELECT holder_name,degree_type,degree_name,issuer_name,passing_year,status
			FROM certificates WHERE uivid=$1`, uivid).
			Scan(&cert.holderName, &cert.degreeType, &cert.degreeName, &cert.issuerName, &cert.passingYear, &cert.status)
		if err == nil {
			found = true
			break
		}
	}

	if !found {
		return map[string]interface{}{
			"request_id": requestID, "uivid": uivid,
			"credential_valid": false, "name_match": false,
			"message":     "❌ CERTIFICATE NOT FOUND. May be fraudulent.",
			"verified_at": now,
		}
	}
	if cert.status == "revoked" {
		return map[string]interface{}{
			"request_id": requestID, "uivid": uivid,
			"credential_valid": false, "name_match": false, "is_revoked": true,
			"message": "⚠️ REVOKED: Certificate has been revoked by issuing institution.",
		}
	}

	nameMatch := similarNames(candidateName, cert.holderName)
	valid := nameMatch

	msg := "✅ VERIFIED — Certificate is authentic."
	if !nameMatch {
		msg = "⚠️ NAME MISMATCH — Certificate exists but name does not match."
		valid = false
	}

	return map[string]interface{}{
		"request_id":       requestID,
		"uivid":            uivid,
		"credential_valid": valid,
		"name_match":       nameMatch,
		"holder_name":      cert.holderName,
		"degree_type":      cert.degreeType,
		"degree_name":      cert.degreeName,
		"institution":      cert.issuerName,
		"passing_year":     cert.passingYear,
		"message":          msg,
		"verified_at":      now,
	}
}

func (h *Handler) History(c *gin.Context) {
	slug := middleware.TenantSlug(c)
	db, _ := h.pool.DB(slug)
	rows, err := db.Query(`SELECT id,uivid,candidate_name_provided,result_valid,verified_at
		FROM verifications ORDER BY verified_at DESC LIMIT 100`)
	if err != nil {
		c.JSON(500, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()
	var list []map[string]interface{}
	for rows.Next() {
		var id, uivid, name string
		var valid bool
		var at time.Time
		rows.Scan(&id, &uivid, &name, &valid, &at)
		list = append(list, map[string]interface{}{
			"id": id, "uivid": uivid, "candidate_name": name,
			"result_valid": valid, "verified_at": at,
		})
	}
	if list == nil {
		list = []map[string]interface{}{}
	}
	c.JSON(200, gin.H{"verifications": list, "count": len(list)})
}

func (h *Handler) Dashboard(c *gin.Context) {
	slug := middleware.TenantSlug(c)
	db, _ := h.pool.DB(slug)
	var total, valid, invalid int
	db.QueryRow("SELECT COUNT(*) FROM verifications").Scan(&total)
	db.QueryRow("SELECT COUNT(*) FROM verifications WHERE result_valid=true").Scan(&valid)
	invalid = total - valid
	c.JSON(200, gin.H{"total_verifications": total, "valid": valid, "invalid": invalid, "fraud_caught": invalid})
}

func (h *Handler) getSlugCandidates(uivid string) []string {
	// In production: query master DB index table
	// For demo: return all known tenant slugs
	return []string{"iit-bombay","mumbai-univ","google-hr","accenture-hr","infosys-hr","naac-reg","fraud-l1"}
}

func similarNames(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b)) ||
		strings.Contains(strings.ToLower(b), strings.ToLower(strings.TrimSpace(a)))
}

