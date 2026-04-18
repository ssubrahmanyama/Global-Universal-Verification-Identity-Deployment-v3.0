// audit.go
package audit

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/uivi/saas/internal/middleware"
	"github.com/uivi/saas/internal/tenant"
)

type Handler struct{ pool *tenant.Pool }

func New(p *tenant.Pool) *Handler { return &Handler{p} }

func (h *Handler) MyTrail(c *gin.Context) {
	slug := middleware.TenantSlug(c)
	uid := middleware.UserID(c)
	db, _ := h.pool.DB(slug)
	rows, _ := db.Query(`SELECT id,action,details,created_at FROM audit_events
		WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100`, uid)
	defer rows.Close()
	var events []map[string]interface{}
	for rows.Next() {
		var id, action, details, at string
		rows.Scan(&id, &action, &details, &at)
		events = append(events, map[string]interface{}{"id": id, "action": action, "details": details, "timestamp": at})
	}
	if events == nil {
		events = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"audit_trail": events, "count": len(events)})
}

func (h *Handler) MyProfile(c *gin.Context) {
	slug := middleware.TenantSlug(c)
	uid := middleware.UserID(c)
	db, _ := h.pool.DB(slug)
	var email, fullName, role, createdAt string
	db.QueryRow("SELECT email,full_name,role,created_at FROM users WHERE id=$1", uid).
		Scan(&email, &fullName, &role, &createdAt)
	c.JSON(200, gin.H{"user_id": uid, "email": email, "full_name": fullName, "role": role,
		"tenant": slug, "member_since": createdAt})
}

func (h *Handler) SetConsent(c *gin.Context) {
	slug := middleware.TenantSlug(c)
	uid := middleware.UserID(c)
	var body struct{ Mode string `json:"mode"` }
	c.ShouldBindJSON(&body)
	db, _ := h.pool.DB(slug)
	db.Exec(`INSERT INTO consent_preferences(user_id,mode,updated_at) VALUES($1,$2,NOW())
		ON CONFLICT(user_id) DO UPDATE SET mode=$2,updated_at=NOW()`, uid, body.Mode)
	c.JSON(200, gin.H{"mode": body.Mode})
}


