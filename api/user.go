package api

import (
	"net/http"

	"regs/db"
	"regs/middleware"
	"regs/model"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type registerRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Register handles POST /api/users/register
// Guest-accessible user registration.
func Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	user := model.User{
		Username: req.Username,
		Password: string(hash),
		Role:     model.RoleUser,
	}

	if err := db.Get().Create(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username already exists"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": user.ID, "username": user.Username, "role": user.Role})
}

// Login handles POST /api/users/login
// Guest-accessible login that returns a JWT token.
func Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user model.User
	if err := db.Get().Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := middleware.SignJWT(jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     string(user.Role),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

// Logout handles POST /api/users/logout
// User-only logout endpoint (currently stateless).
func Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "logged out"})
}

// Me handles GET /api/users/me
// User-only current user profile lookup.
func Me(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	var user model.User
	if err := db.Get().First(&user, userID).Error; err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"role":     user.Role,
	})
}

// UserSubmissions handles GET /api/users/:user_id/submissions
// Guest-accessible lookup of a user's submission history.
func UserSubmissions(c *gin.Context) {
	userIDParam := c.Param("user_id")
	var user model.User
	if err := db.Get().First(&user, userIDParam).Error; err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	var submissions []model.Submission
	if err := db.Get().Where("user_id = ?", user.ID).Find(&submissions).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, submissions)
}

type promoteRequest struct {
	Username string `json:"username" binding:"required"`
}

// PromoteToAdmin handles POST /api/users/promote
// Admin-only promotion to admin role.
func PromoteToAdmin(c *gin.Context) {
	var req promoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user model.User
	if err := db.Get().Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	if err := db.Get().Model(&user).Update("role", model.RoleAdmin).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to promote user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"username": user.Username, "role": "admin"})
}

// ListUsers handles GET /api/users
// Admin-only listing of all users.
func ListUsers(c *gin.Context) {
	var users []model.User
	if err := db.Get().Find(&users).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, users)
}
