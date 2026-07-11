package handlers

import (
	"net/http"
	"strings"

	"animalpoke/backend/internal/cases"
	"animalpoke/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

// CaseHandler serves AP-086 unified case APIs.
type CaseHandler struct {
	svc *cases.Service
}

// NewCaseHandler constructs handler.
func NewCaseHandler(svc *cases.Service) *CaseHandler {
	if svc == nil {
		svc = cases.Default()
	}
	return &CaseHandler{svc: svc}
}

func actor(c *gin.Context) string {
	if v, ok := c.Get("admin_id"); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	if v, ok := c.Get("device_id"); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return "unknown"
}

func roleFromHeader(c *gin.Context) cases.Role {
	switch strings.ToLower(c.GetHeader("X-Admin-Role")) {
	case "admin":
		return cases.RoleAdmin
	case "support":
		return cases.RoleSupport
	default:
		// JWT path without admin role → treat as support for admin routes is wrong;
		// user routes use UserStatus.
		return cases.RoleSupport
	}
}

// CreateCase POST /api/v1/admin/cases
func (h *CaseHandler) CreateCase(c *gin.Context) {
	var body struct {
		ResourceType  string `json:"resource_type"`
		ResourceID    string `json:"resource_id"`
		ReporterEmail string `json:"reporter_email"`
		SLAHours      int    `json:"sla_hours"`
	}
	if err := middleware.BindStrictJSON(c, &body); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	cs, err := h.svc.Create(body.ResourceType, body.ResourceID, actor(c), body.ReporterEmail, body.SLAHours)
	if err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "case_invalid", err.Error(), false, nil)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"case": cs, "request_id": middleware.GetRequestID(c)})
}

// ListCases GET /api/v1/admin/cases
func (h *CaseHandler) ListCases(c *gin.Context) {
	list := h.svc.List(c.Query("state"), roleFromHeader(c))
	c.JSON(http.StatusOK, gin.H{"cases": list, "request_id": middleware.GetRequestID(c)})
}

// GetCase GET /api/v1/admin/cases/:id
func (h *CaseHandler) GetCase(c *gin.Context) {
	cs, err := h.svc.Get(c.Param("id"), actor(c), roleFromHeader(c))
	if err != nil {
		writeCaseErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"case": cs, "request_id": middleware.GetRequestID(c)})
}

// AssignCase POST /api/v1/admin/cases/:id/assign
func (h *CaseHandler) AssignCase(c *gin.Context) {
	var body struct {
		Assignee string `json:"assignee"`
	}
	if err := middleware.BindStrictJSON(c, &body); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	if body.Assignee == "" {
		body.Assignee = actor(c)
	}
	cs, err := h.svc.Assign(c.Param("id"), actor(c), body.Assignee)
	if err != nil {
		writeCaseErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"case": cs, "request_id": middleware.GetRequestID(c)})
}

// TransitionCase POST /api/v1/admin/cases/:id/transition
func (h *CaseHandler) TransitionCase(c *gin.Context) {
	var body struct {
		State  string `json:"state"`
		Reason string `json:"reason"`
	}
	if err := middleware.BindStrictJSON(c, &body); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	cs, err := h.svc.Transition(c.Param("id"), actor(c), body.State, body.Reason)
	if err != nil {
		writeCaseErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"case": cs, "request_id": middleware.GetRequestID(c)})
}

// AddNote POST /api/v1/admin/cases/:id/notes
func (h *CaseHandler) AddNote(c *gin.Context) {
	var body struct {
		Body string `json:"body"`
	}
	if err := middleware.BindStrictJSON(c, &body); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	cs, err := h.svc.AddNote(c.Param("id"), actor(c), body.Body)
	if err != nil {
		writeCaseErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"case": cs, "request_id": middleware.GetRequestID(c)})
}

// ListAudits GET /api/v1/admin/cases/:id/audits
func (h *CaseHandler) ListAudits(c *gin.Context) {
	// ensure case exists + view audit
	if _, err := h.svc.Get(c.Param("id"), actor(c), roleFromHeader(c)); err != nil {
		writeCaseErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"audits": h.svc.Audits(c.Param("id")), "request_id": middleware.GetRequestID(c)})
}

// UserCaseStatus GET /api/v1/cases/:id
func (h *CaseHandler) UserCaseStatus(c *gin.Context) {
	reporter := actor(c)
	cs, err := h.svc.UserStatus(c.Param("id"), reporter)
	if err != nil {
		writeCaseErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"case": cs, "request_id": middleware.GetRequestID(c)})
}

func writeCaseErr(c *gin.Context, err error) {
	if cases.IsNotFound(err) {
		middleware.WriteError(c, http.StatusNotFound, "case_not_found", err.Error(), false, nil)
		return
	}
	if cases.IsConflict(err) {
		middleware.WriteError(c, http.StatusConflict, "case_conflict", err.Error(), false, nil)
		return
	}
	middleware.WriteError(c, http.StatusBadRequest, "case_error", err.Error(), false, nil)
}
