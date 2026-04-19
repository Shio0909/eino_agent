package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	ctxUserIDKey   = "user_id"
	ctxUserRoleKey = "user_role"
	ctxTenantIDKey = "tenant_id"
)

type tokenClaims struct {
	Sub      string `json:"sub"`
	Role     string `json:"role"`
	TenantID int    `json:"tenant_id"`
	Exp      int64  `json:"exp"`
	Iat      int64  `json:"iat"`
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Login(c *gin.Context) {
	if !h.cfg.Auth.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "auth 未启用，请先设置 AUTH_ENABLED=true"})
		return
	}

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	role := ""
	tenantID := 0
	if h.cfg.Auth.AdminPassword != "" && req.Username == h.cfg.Auth.AdminUsername && req.Password == h.cfg.Auth.AdminPassword {
		role = "admin"
		tenantID = h.cfg.Auth.AdminTenantID
	} else if h.cfg.Auth.UserPassword != "" && req.Username == h.cfg.Auth.UserUsername && req.Password == h.cfg.Auth.UserPassword {
		role = "user"
		tenantID = h.cfg.Auth.UserTenantID
	}

	if role == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	if tenantID <= 0 {
		tenantID = 1
	}

	now := time.Now().Unix()
	expiresAt := time.Now().Add(time.Duration(h.cfg.Auth.AccessTokenExpireMinutes) * time.Minute).Unix()
	claims := tokenClaims{
		Sub:      req.Username,
		Role:     role,
		TenantID: tenantID,
		Exp:      expiresAt,
		Iat:      now,
	}
	accessToken, err := h.signToken(claims)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成 token 失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   h.cfg.Auth.AccessTokenExpireMinutes * 60,
		"user": gin.H{
			"id":        req.Username,
			"role":      role,
			"tenant_id": tenantID,
		},
	})
}

func (h *Handler) Me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":        h.getUserID(c),
			"role":      h.getUserRole(c),
			"tenant_id": h.getTenantID(c),
		},
	})
}

func (h *Handler) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !h.cfg.Auth.Enabled {
			c.Set(ctxUserIDKey, "anonymous")
			c.Set(ctxUserRoleKey, "admin")
			c.Set(ctxTenantIDKey, 1)
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "缺少 Bearer token"})
			return
		}

		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		claims, err := h.parseToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		c.Set(ctxUserIDKey, claims.Sub)
		c.Set(ctxUserRoleKey, claims.Role)
		c.Set(ctxTenantIDKey, claims.TenantID)
		c.Next()
	}
}

func (h *Handler) RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !h.cfg.Auth.Enabled {
			c.Next()
			return
		}

		currentRole := h.getUserRole(c)
		for _, r := range roles {
			if currentRole == r {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "权限不足"})
	}
}

func (h *Handler) getTenantID(c *gin.Context) int {
	if v, ok := c.Get(ctxTenantIDKey); ok {
		if tenantID, ok := v.(int); ok {
			return tenantID
		}
	}
	return 1
}

// parsePagination extracts page/page_size from query params.
// defaultSize: default page size; maxSize: upper cap on page size.
func parsePagination(c *gin.Context, defaultSize, maxSize int) (page, pageSize int) {
	page = 1
	pageSize = defaultSize
	if v, err := strconv.Atoi(c.Query("page")); err == nil && v >= 1 {
		page = v
	}
	if v, err := strconv.Atoi(c.Query("page_size")); err == nil && v >= 1 {
		if v > maxSize {
			v = maxSize
		}
		pageSize = v
	}
	return
}

func (h *Handler) getUserRole(c *gin.Context) string {
	if v, ok := c.Get(ctxUserRoleKey); ok {
		if role, ok := v.(string); ok {
			return role
		}
	}
	return "anonymous"
}

func (h *Handler) getUserID(c *gin.Context) string {
	if v, ok := c.Get(ctxUserIDKey); ok {
		if userID, ok := v.(string); ok {
			return userID
		}
	}
	return "anonymous"
}

func (h *Handler) signToken(claims tokenClaims) (string, error) {
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerBytes, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	enc := base64.RawURLEncoding
	headerSeg := enc.EncodeToString(headerBytes)
	payloadSeg := enc.EncodeToString(payloadBytes)
	unsigned := headerSeg + "." + payloadSeg
	sig := h.signHMAC(unsigned)
	return unsigned + "." + sig, nil
}

func (h *Handler) parseToken(token string) (*tokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("token 格式非法")
	}

	unsigned := parts[0] + "." + parts[1]
	expectedSig := h.signHMAC(unsigned)
	if subtle.ConstantTimeCompare([]byte(parts[2]), []byte(expectedSig)) != 1 {
		return nil, fmt.Errorf("token 签名无效")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("token 载荷非法")
	}

	var claims tokenClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("token 载荷解析失败")
	}
	if claims.Exp <= time.Now().Unix() {
		return nil, fmt.Errorf("token 已过期")
	}
	if claims.Sub == "" {
		return nil, fmt.Errorf("token 缺少主体")
	}
	if claims.TenantID <= 0 {
		claims.TenantID = 1
	}
	if claims.Role == "" {
		claims.Role = "user"
	}
	return &claims, nil
}

func (h *Handler) signHMAC(input string) string {
	mac := hmac.New(sha256.New, []byte(h.cfg.Auth.JWTSecret))
	_, _ = mac.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
