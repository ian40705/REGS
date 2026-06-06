package api

import (
	"strconv"

	"regs/model"

	"github.com/gin-gonic/gin"
)

func currentUserID(c *gin.Context) (uint, bool) {
	userIDValue, ok := c.Get("user_id")
	if !ok {
		return 0, false
	}

	switch v := userIDValue.(type) {
	case float64:
		return uint(v), true
	case float32:
		return uint(v), true
	case int:
		return uint(v), true
	case int64:
		return uint(v), true
	case uint:
		return v, true
	case string:
		u, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return 0, false
		}
		return uint(u), true
	}

	return 0, false
}

func currentUsername(c *gin.Context) string {
	if username, ok := c.Get("username"); ok {
		if s, ok := username.(string); ok {
			return s
		}
	}
	return ""
}

func currentUserRole(c *gin.Context) model.Role {
	if role, ok := c.Get("role"); ok {
		if s, ok := role.(string); ok {
			return model.Role(s)
		}
	}
	return model.RoleGuest
}
