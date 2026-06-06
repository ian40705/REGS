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
}

type submissionListResponse struct {
	OperatorID string    `json:"operatorId"`
	ProblemID  uint      `json:"problemId"`
	Status     string    `json:"status"`
	ExitCode   int       `json:"exitCode"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Submit handles POST /api/submissions
// User-only submission endpoint for uploading code to a problem.
// Returns immediately with operatorId; judging happens asynchronously in background.
func Submit(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	problemIDStr := c.PostForm("problem_id")
	problemID, err := strconv.ParseUint(problemIDStr, 10, 64)
	if err != nil || problemID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "problem_id is required"})
		return
	}

	if err := db.Get().First(&model.Problem{}, uint(problemID)).Error; err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	file, err := c.FormFile("zip")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "zip file is required"})
		return
	}

	operatorID := uuid.New().String()
	dir := "uploads"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create upload directory"})
		return
	}

	path := filepath.Join(dir, operatorID+".zip")
	if err := c.SaveUploadedFile(file, path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save upload"})
		return
	}

	submission := model.Submission{
		OperatorID: operatorID,
		UserID:     userID,
		ProblemID:  uint(problemID),
		Status:     "pending",
		ExitCode:   0,
		SourcePath: path,
		CreatedAt:  time.Now(),
	}
	if err := db.Get().Create(&submission).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record submission"})
		return
	}

	// Enqueue judging task (returns immediately, judging happens in background)
	task := service.JudgeTask{
		OperatorID:   operatorID,
		ZipPath:      path,
		ProblemID:    uint(problemID),
		TimeLimitSec: 5,
	}
	if err := service.GlobalQueue.Enqueue(task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to enqueue task"})
		return
	}

	// Return immediately with operatorId
	c.JSON(http.StatusCreated, gin.H{
		"operatorId": operatorID,
		"status":     submission.Status,
	})
}

// ListSubmissions handles GET /api/submissions
// User-only listing of the current user's submissions.
func ListSubmissions(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	var submissions []model.Submission
	if err := db.Get().Where("user_id = ?", userID).Find(&submissions).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	response := make([]submissionListResponse, 0, len(submissions))
	for _, s := range submissions {
		response = append(response, submissionListResponse{
			OperatorID: s.OperatorID,
			ProblemID:  s.ProblemID,
			Status:     s.Status,
			ExitCode:   s.ExitCode,
			CreatedAt:  s.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, response)
}

// GetSubmission handles GET /api/submissions/:operatorId
// User-only retrieval of a submission record; admin may access any submission.
func GetSubmission(c *gin.Context) {
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

	c.JSON(http.StatusOK, submission)
}

// GetSubmissionSource handles GET /api/submissions/:operatorId/source
// User-only download of submission source; admin may access any submission.
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

	if _, err := os.Stat(submission.SourcePath); err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	c.FileAttachment(submission.SourcePath, filepath.Base(submission.SourcePath))
}

// Result handles GET /api/submissions/:operatorId
// Public result lookup for a submission by operator ID.
func Result(c *gin.Context) {
	operatorID := c.Param("operatorId")
	var submission model.Submission
	if err := db.Get().Where("operator_id = ?", operatorID).First(&submission).Error; err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, JudgeResult{
		OperatorID: submission.OperatorID,
		Status:     submission.Status,
		ExitCode:   submission.ExitCode,
	})
}
