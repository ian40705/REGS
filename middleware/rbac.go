package middleware

import (
	"net/http"

	"regs/model"

	"github.com/gin-gonic/gin"
)

type Permission string

const (
	PermSubmit   Permission = "submission:create"
	PermView     Permission = "submission:read"
	PermAdminAll Permission = "admin:all"
)

var rolePermissions = map[model.Role][]Permission{
	model.RoleGuest: {},
	model.RoleUser: {
		PermSubmit,
		PermView,
	},
	model.RoleAdmin: {
		PermSubmit,
		PermView,
		PermAdminAll,
	},
}

func roleHasPermission(role model.Role, permission Permission) bool {
	perms, ok := rolePermissions[role]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == permission {
			return true
		}
	}
	return false
}

// RequireRole enforces that the current JWT-authenticated user has at least the specified role.
func RequireRole(level model.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := model.Role(c.GetString("role"))

		switch level {
		case model.RoleGuest:
			c.Next()
			return
		case model.RoleUser:
			if role == model.RoleUser || role == model.RoleAdmin {
				c.Next()
				return
			}
		case model.RoleAdmin:
			if role == model.RoleAdmin {
				c.Next()
				return
			}
		}

		c.AbortWithStatus(http.StatusForbidden)
	}
}

// RequirePermission enforces permission-based access control for routes.
// It maps JWT role claims to the permissions required by each endpoint.
func RequirePermission(permission Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := model.Role(c.GetString("role"))
		if roleHasPermission(role, permission) {
			c.Next()
			return
		}
		c.AbortWithStatus(http.StatusForbidden)
	}
}
