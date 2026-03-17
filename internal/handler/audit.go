package handler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type AuditLogger struct {
	mu   sync.Mutex
	path string
}

type auditEvent struct {
	Time     string                 `json:"time"`
	Action   string                 `json:"action"`
	Resource string                 `json:"resource"`
	Success  bool                   `json:"success"`
	UserID   string                 `json:"user_id"`
	Role     string                 `json:"role"`
	TenantID int                    `json:"tenant_id"`
	IP       string                 `json:"ip"`
	Method   string                 `json:"method"`
	Path     string                 `json:"path"`
	Details  map[string]interface{} `json:"details,omitempty"`
}

func NewAuditLogger(path string) (*AuditLogger, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &AuditLogger{path: path}, nil
}

func (l *AuditLogger) Write(event auditEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	line, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = f.Write(append(line, '\n'))
	return err
}

func (h *Handler) audit(c *gin.Context, action, resource string, success bool, details map[string]interface{}) {
	if h.auditLogger == nil {
		return
	}
	_ = h.auditLogger.Write(auditEvent{
		Time:     time.Now().Format(time.RFC3339),
		Action:   action,
		Resource: resource,
		Success:  success,
		UserID:   h.getUserID(c),
		Role:     h.getUserRole(c),
		TenantID: h.getTenantID(c),
		IP:       c.ClientIP(),
		Method:   c.Request.Method,
		Path:     c.Request.URL.Path,
		Details:  details,
	})
}
