package api

import (
	"net/http"
	"strconv"

	"regs/db"
	"regs/model"

	"github.com/gin-gonic/gin"
)

// ProblemStats handles GET /api/stats/problems/:problem_id
// Guest-accessible aggregated statistics for a problem.
func ProblemStats(c *gin.Context) {
	problemID, err := strconv.ParseUint(c.Param("problem_id"), 10, 64)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	dbConn := db.Get()
	var total int64
	if err := dbConn.Model(&model.Submission{}).Where("problem_id = ?", uint(problemID)).Count(&total).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	var distinctUsers int64
	dbConn.Model(&model.Submission{}).Select("count(distinct user_id)").Where("problem_id = ?", uint(problemID)).Scan(&distinctUsers)

	var accepted int64
	dbConn.Model(&model.Submission{}).Where("problem_id = ? AND status = ?", uint(problemID), "AC").Count(&accepted)

	successRate := float64(0)
	if total > 0 {
		successRate = float64(accepted) / float64(total)
	}

	c.JSON(http.StatusOK, gin.H{
		"problem_id":   problemID,
		"submissions":  total,
		"users":        distinctUsers,
		"accepted":     accepted,
		"success_rate": successRate,
	})
}

// UserStats handles GET /api/stats/users/:user_id
// Guest-accessible aggregated statistics for a user.
// Accepts either a numeric user ID or a username string.
func UserStats(c *gin.Context) {
	param := c.Param("user_id")

	dbConn := db.Get()

	// Resolve the user: try numeric ID first, then username lookup.
	var user model.User
	userID, err := strconv.ParseUint(param, 10, 64)
	if err != nil {
		// param is a username string
		if err2 := dbConn.Where("username = ?", param).First(&user).Error; err2 != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		userID = uint64(user.ID)
	}

	var total int64
	if err := dbConn.Model(&model.Submission{}).Where("user_id = ?", uint(userID)).Count(&total).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	var distinctProblems int64
	dbConn.Model(&model.Submission{}).Select("count(distinct problem_id)").Where("user_id = ?", uint(userID)).Scan(&distinctProblems)

	var accepted int64
	dbConn.Model(&model.Submission{}).Where("user_id = ? AND status = ?", uint(userID), "AC").Count(&accepted)

	successRate := float64(0)
	if total > 0 {
		successRate = float64(accepted) / float64(total)
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":        userID,
		"submissions":    total,
		"problems_tried": distinctProblems,
		"accepted":       accepted,
		"success_rate":   successRate,
	})
}