package api

import (
	"net/http"
	"os"
	"strconv"

	"regs/db"
	"regs/model"

	"github.com/gin-gonic/gin"
)

type problemRequest struct {
	ID           uint            `json:"id"`
	Title        string          `json:"title" binding:"required"`
	Description  string          `json:"description" binding:"required"`
	TestcasePath string          `json:"testcase_path"`
	// JudgeType: "stdio"（預設）或 "catch2"（114Final 題型）
	JudgeType    model.JudgeType `json:"judge_type"`
	// MaxScore: catch2 模式下的滿分，由管理員填入（或系統自動計算）
	MaxScore     int             `json:"max_score"`
}

func ListProblems(c *gin.Context) {
	var problems []model.Problem
	if err := db.Get().Find(&problems).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, problems)
}

func GetProblem(c *gin.Context) {
	problemID, err := strconv.ParseUint(c.Param("problem_id"), 10, 64)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	var problem model.Problem
	if err := db.Get().First(&problem, uint(problemID)).Error; err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, problem)
}

func UpsertProblem(c *gin.Context) {
	var req problemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 預設 JudgeType
	if req.JudgeType == "" {
		req.JudgeType = model.JudgeTypeStdio
	}

	var problem model.Problem
	if req.ID != 0 {
		if err := db.Get().First(&problem, req.ID).Error; err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
	} else {
		if err := db.Get().Where("title = ?", req.Title).First(&problem).Error; err != nil {
			problem = model.Problem{Title: req.Title}
		}
	}

	problem.Title = req.Title
	problem.Description = req.Description
	problem.TestcasePath = req.TestcasePath
	problem.JudgeType = req.JudgeType
	problem.MaxScore = req.MaxScore

	if problem.ID == 0 {
		if err := db.Get().Create(&problem).Error; err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	} else {
		if err := db.Get().Save(&problem).Error; err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	}

	c.JSON(http.StatusOK, problem)
}

func DeleteProblem(c *gin.Context) {
	problemID, err := strconv.ParseUint(c.Param("problem_id"), 10, 64)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	var problem model.Problem
	if err := db.Get().First(&problem, uint(problemID)).Error; err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if err := db.Get().Delete(&problem).Error; err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusNoContent)
}

func DownloadProblemTestcases(c *gin.Context) {
	problemID, err := strconv.ParseUint(c.Param("problem_id"), 10, 64)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	var problem model.Problem
	if err := db.Get().First(&problem, uint(problemID)).Error; err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if problem.TestcasePath == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if _, err := os.Stat(problem.TestcasePath); err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	c.FileAttachment(problem.TestcasePath, "testcases.zip")
}
