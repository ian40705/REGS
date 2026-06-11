package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"regs/db"
	"regs/model"
	"regs/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type JudgeResult struct {
	OperatorID string `json:"operatorId"`
	Status     string `json:"status"`
	ExitCode   int    `json:"exitCode"`
	Score      int    `json:"score"`
}

type submissionListResponse struct {
	OperatorID string    `json:"operatorId"`
	ProblemID  uint      `json:"problemId"`
	Status     string    `json:"status"`
	ExitCode   int       `json:"exitCode"`
	CreatedAt  time.Time `json:"createdAt"`
}

// ----------------------------
// Submit
// ----------------------------
func Submit(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	problemIDStr := c.PostForm("problem_id")
	problemID, err := strconv.ParseUint(problemIDStr, 10, 64)
	if err != nil || problemID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid problem_id"})
		return
	}

	var problem model.Problem
	if err := db.Get().First(&problem, uint(problemID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "problem not found"})
		return
	}

	file, err := c.FormFile("zip")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "zip required"})
		return
	}

	operatorID := uuid.New().String()

	dir := "uploads"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mkdir failed"})
		return
	}

	path := filepath.Join(dir, operatorID+".zip")
	if err := c.SaveUploadedFile(file, path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save failed"})
		return
	}

	submission := model.Submission{
		OperatorID: operatorID,
		UserID:     userID,
		ProblemID:  uint(problemID),
		Status:     "pending",
		ExitCode:   0,
		Score:      0,
		SourcePath: path,
		CreatedAt:  time.Now(),
	}

	if err := db.Get().Create(&submission).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db insert failed"})
		return
	}

	task := service.JudgeTask{
		OperatorID:   operatorID,
		ZipPath:      path,
		ProblemID:    uint(problemID),
		TimeLimitSec: 5,
	}

	if err := service.GlobalQueue.Enqueue(task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "queue full"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"operatorId": operatorID,
		"status":     "pending",
	})
}

// ----------------------------
// List submissions
// ----------------------------
func ListSubmissions(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var submissions []model.Submission
	if err := db.Get().
		Where("user_id = ?", userID).
		Order("created_at desc").
		Find(&submissions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db error"})
		return
	}

	resp := make([]submissionListResponse, 0, len(submissions))
	for _, s := range submissions {
		resp = append(resp, submissionListResponse{
			OperatorID: s.OperatorID,
			ProblemID:  s.ProblemID,
			Status:     s.Status,
			ExitCode:   s.ExitCode,
			CreatedAt:  s.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, resp)
}

// ----------------------------
// Get submission
// ----------------------------
func GetSubmission(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	operatorID := c.Param("operatorId")

	var submission model.Submission
	if err := db.Get().
		Where("operator_id = ?", operatorID).
		First(&submission).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	if submission.UserID != userID && currentUserRole(c) != model.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	c.JSON(http.StatusOK, submission)
}

// ----------------------------
// Get logs (FIXED: no heuristic fallback needed anymore if you store workspace path)
// ----------------------------
func GetSubmissionLogs(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	operatorID := c.Param("operatorId")

	var submission model.Submission
	if err := db.Get().
		Where("operator_id = ?", operatorID).
		First(&submission).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	if submission.UserID != userID && currentUserRole(c) != model.RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	workspace := filepath.Join("uploads", "workspace-"+operatorID)

	read := func(name string) string {
		p := filepath.Join(workspace, name)
		b, err := os.ReadFile(p)
		if err != nil {
			return ""
		}
		return string(b)
	}

	c.JSON(http.StatusOK, gin.H{
		"configure": read("configure.log"),
		"compile":   read("compile.log"),
		"output":    read("output.log"),
	})
}

// ----------------------------
// Result
// ----------------------------
func Result(c *gin.Context) {
	operatorID := c.Param("operatorId")

	var submission model.Submission
	if err := db.Get().
		Where("operator_id = ?", operatorID).
		First(&submission).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	c.JSON(http.StatusOK, JudgeResult{
		OperatorID: submission.OperatorID,
		Status:     submission.Status,
		ExitCode:   submission.ExitCode,
		Score:      submission.Score,
	})
}

func GetSubmissionSource(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	operatorID := c.Param("operatorId")

	var submission model.Submission
	if err := db.Get().Where("operator_id = ?", operatorID).First(&submission).Error; err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if submission.UserID != userID && currentUserRole(c) != model.RoleAdmin {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	c.FileAttachment(submission.SourcePath, filepath.Base(submission.SourcePath))
}