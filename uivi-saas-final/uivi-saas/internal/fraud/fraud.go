package fraud

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/uivi/saas/internal/middleware"
	"github.com/uivi/saas/internal/tenant"
)

type Handler struct {
	pool     *tenant.Pool
	masterDB *sql.DB
}

func New(p *tenant.Pool, masterDB *sql.DB) *Handler { return &Handler{p, masterDB} }

func (h *Handler) Dashboard(c *gin.Context) {
	slug := middleware.TenantSlug(c)
	db, _ := h.pool.DB(slug)
	var total, flagged, critical int
	db.QueryRow("SELECT COUNT(*) FROM fraud_checks").Scan(&total)
	db.QueryRow("SELECT COUNT(*) FROM fraud_checks WHERE flagged=true").Scan(&flagged)
	db.QueryRow("SELECT COUNT(*) FROM fraud_checks WHERE risk_level='CRITICAL'").Scan(&critical)
	c.JSON(200, gin.H{
		"total_checks": total, "flagged": flagged, "critical": critical,
		"clean": total - flagged, "last_updated": time.Now(),
	})
}

func (h *Handler) Alerts(c *gin.Context) {
	slug := middleware.TenantSlug(c)
	db, _ := h.pool.DB(slug)
	rows, _ := db.Query(`SELECT id,uivid,risk_score,risk_level,signals,status,created_at
		FROM fraud_checks WHERE flagged=true ORDER BY created_at DESC LIMIT 50`)
	defer rows.Close()
	var alerts []map[string]interface{}
	for rows.Next() {
		var id, uivid, level, signals, status, createdAt string
		var score int
		rows.Scan(&id, &uivid, &score, &level, &signals, &status, &createdAt)
		alerts = append(alerts, map[string]interface{}{
			"id": id, "uivid": uivid, "risk_score": score,
			"risk_level": level, "signals": signals,
			"status": status, "created_at": createdAt,
		})
	}
	if alerts == nil {
		alerts = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"alerts": alerts, "count": len(alerts)})
}

func (h *Handler) Report(c *gin.Context) {
	slug := middleware.TenantSlug(c)
	id := c.Param("id")
	var body struct{ Action, Notes string }
	c.ShouldBindJSON(&body)
	db, _ := h.pool.DB(slug)
	db.Exec("UPDATE fraud_checks SET status=$1,notes=$2 WHERE id=$3", body.Action, body.Notes, id)
	c.JSON(200, gin.H{"message": "fraud report updated"})
}

func (h *Handler) Graph(c *gin.Context) {
	slug := middleware.TenantSlug(c)
	db, _ := h.pool.DB(slug)
	rows, _ := db.Query("SELECT source_type,source_id,target_type,target_id FROM fraud_graph LIMIT 200")
	defer rows.Close()
	var nodes, edges []map[string]interface{}
	seen := map[string]bool{}
	for rows.Next() {
		var st, si, tt, ti string
		rows.Scan(&st, &si, &tt, &ti)
		if !seen[si] { nodes = append(nodes, map[string]interface{}{"id": si, "type": st}); seen[si] = true }
		if !seen[ti] { nodes = append(nodes, map[string]interface{}{"id": ti, "type": tt}); seen[ti] = true }
		edges = append(edges, map[string]interface{}{"from": si, "to": ti})
	}
	c.JSON(200, gin.H{"nodes": nodes, "edges": edges})
}
