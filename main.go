package main

import (
	"log"
	"os"
	"regs/api"
	"regs/db"
	"regs/middleware"
	"regs/model"
	"regs/service"

	"github.com/gin-gonic/gin"
)

func main() {
	// init DB (DSN from env var REGS_DATABASE_DSN, fallback to sqlite file)
	dsn := os.Getenv("REGS_DATABASE_DSN")
	if dsn == "" {
		dsn = "data/regs.db"
	}
	db.MustInit(dsn)
	if err := model.Migrate(db.Get()); err != nil {
		log.Fatalf("migrate failed: %v", err)
	}

	if err := model.SeedDefaults(db.Get()); err != nil {
		log.Fatalf("seeding defaults failed: %v", err)
	}

	if err := model.SeedDefaultAdmin(db.Get()); err != nil {
		log.Fatalf("seeding default admin failed: %v", err)
	}

	// Initialize task queue with 4 worker goroutines
	service.InitQueue(4)

	// load public key for JWT verification (optional)
	if err := middleware.LoadPublicKeyFromFile("public.pem"); err != nil {
		// warn but continue; you may provide a public.pem for ECDSA verification
		log.Printf("warning: could not load public.pem: %v", err)
	}

	r := gin.Default()

	apiGroup := r.Group("/api")

	// UserInfo endpoints
	apiGroup.POST("/users/register", api.Register)                                                           // Guest: register new user
	apiGroup.POST("/users/login", api.Login)                                                                 // Guest: login and receive JWT
	apiGroup.POST("/users/logout", middleware.JWTAuth(), middleware.RequireRole(model.RoleUser), api.Logout) // User: logout
	apiGroup.GET("/users/me", middleware.JWTAuth(), middleware.RequireRole(model.RoleUser), api.Me)          // User: current user info
	apiGroup.GET("/users/:user_id/submissions", api.UserSubmissions)                                         // Guest: list user submissions

	// Problem endpoints
	apiGroup.GET("/problems", api.ListProblems)                                                                                                                // Guest: list problems
	apiGroup.GET("/problems/:problem_id", api.GetProblem)                                                                                                      // Guest: get problem details
	apiGroup.PUT("/problems", middleware.JWTAuth(), middleware.RequirePermission(middleware.PermAdminAll), api.UpsertProblem)                                  // Admin: create/update problem
	apiGroup.DELETE("/problems/:problem_id", middleware.JWTAuth(), middleware.RequirePermission(middleware.PermAdminAll), api.DeleteProblem)                   // Admin: delete problem
	apiGroup.GET("/problems/:problem_id/testcases", middleware.JWTAuth(), middleware.RequirePermission(middleware.PermAdminAll), api.DownloadProblemTestcases) // Admin: download testcases

	// Submission endpoints
	apiGroup.POST("/submissions", middleware.JWTAuth(), middleware.RequirePermission(middleware.PermSubmit), api.Submit)                              // User: submit code
	apiGroup.GET("/submissions", middleware.JWTAuth(), middleware.RequirePermission(middleware.PermView), api.ListSubmissions)                        // User: list own submissions
	apiGroup.GET("/submissions/:operatorId/source", middleware.JWTAuth(), middleware.RequirePermission(middleware.PermView), api.GetSubmissionSource) // User: download submission source
	apiGroup.GET("/submissions/:operatorId", middleware.JWTAuth(), middleware.RequirePermission(middleware.PermView), api.GetSubmission)              // User: get submission result

	// Statistics endpoints
	apiGroup.GET("/stats/problems/:problem_id", api.ProblemStats) // Guest: problem statistics
	apiGroup.GET("/stats/users/:user_id", api.UserStats)          // Guest: user statistics

	// Admin endpoints
	apiGroup.POST("/users/promote", middleware.JWTAuth(), middleware.RequireRole(model.RoleAdmin), api.PromoteToAdmin) // Admin: promote user to admin
	apiGroup.GET("/users", middleware.JWTAuth(), middleware.RequireRole(model.RoleAdmin), api.ListUsers)               // Admin: list all users
	r.Run(":8080")
}
