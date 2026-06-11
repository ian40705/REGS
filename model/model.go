package model

import (
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Role string

const (
	RoleGuest Role = "guest"
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type User struct {
	ID       uint   `gorm:"primaryKey"`
	Username string `gorm:"uniqueIndex;not null"`
	Password string `gorm:"not null"`
	Role     Role   `gorm:"type:varchar(20);not null"`
}

// JudgeType 區分題目評測模式
type JudgeType string

const (
	// JudgeTypeStdio：傳統 stdin/stdout 模式（原有）
	JudgeTypeStdio JudgeType = "stdio"
	// JudgeTypeCatch2：Catch2 多測資模式（新增，用於 114Final 題目）
	JudgeTypeCatch2 JudgeType = "catch2"
	// JudgeTypeCMake：CMake pipeline 模式（catch2 的別名，相容 test_all.sh）
	JudgeTypeCMake JudgeType = "cmake"
)

type Problem struct {
	ID           uint      `gorm:"primaryKey"`
	Title        string    `gorm:"uniqueIndex;not null" json:"title"`
	Description  string    `gorm:"type:text;not null" json:"description"`
	TestcasePath string    `gorm:"type:text" json:"testcase_path"`
	// JudgeType 決定使用哪個 judge script。預設 "stdio" 保持向下相容。
	JudgeType    JudgeType `gorm:"type:varchar(20);not null;default:'stdio'" json:"judge_type"`
	// MaxScore：Catch2 模式下的滿分（由 scores.txt 加總，供 API 回傳）
	MaxScore     int       `gorm:"default:0" json:"max_score"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Submission struct {
	OperatorID string    `gorm:"primaryKey;type:varchar(36)" json:"operator_id"`
	UserID     uint      `json:"user_id"`
	User       User      `gorm:"foreignKey:UserID;constraint:OnDelete:SET NULL" json:"-"`
	ProblemID  uint      `json:"problem_id"`
	Problem    Problem   `gorm:"foreignKey:ProblemID;constraint:OnDelete:SET NULL" json:"-"`
	Status     string    `gorm:"type:varchar(50);not null" json:"status"`
	ExitCode   int       `json:"exit_code"`
	// Score：Catch2 模式下的實際得分（stdio 模式下為 0）
	Score      int       `gorm:"default:0" json:"score"`
	SourcePath string    `gorm:"type:text;not null" json:"source_path"`
	CreatedAt  time.Time `json:"created_at"`
}

type Permission struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"uniqueIndex;not null"`
}

type RolePermission struct {
	ID           uint `gorm:"primaryKey"`
	Role         Role `gorm:"type:varchar(20);index;not null"`
	PermissionID uint
	Permission   Permission `gorm:"foreignKey:PermissionID;constraint:OnDelete:CASCADE"`
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&User{}, &Permission{}, &RolePermission{}, &Problem{}, &Submission{})
}

func SeedDefaults(db *gorm.DB) error {
	perms := []string{"submission:create", "submission:read", "admin:all"}

	for _, name := range perms {
		var p Permission
		if err := db.Where("name = ?", name).First(&p).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := db.Create(&Permission{Name: name}).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}

	roleMap := map[Role][]string{
		RoleUser:  {"submission:create", "submission:read"},
		RoleAdmin: {"submission:create", "submission:read", "admin:all"},
	}

	for role, names := range roleMap {
		for _, name := range names {
			var perm Permission
			if err := db.Where("name = ?", name).First(&perm).Error; err != nil {
				return err
			}
			var rp RolePermission
			if err := db.Where("role = ? AND permission_id = ?", role, perm.ID).First(&rp).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					if err := db.Create(&RolePermission{Role: role, PermissionID: perm.ID}).Error; err != nil {
						return err
					}
				} else {
					return err
				}
			}
		}
	}

	return nil
}

func SeedDefaultAdmin(db *gorm.DB) error {
	var count int64
	if err := db.Model(&User{}).Where("role = ?", RoleAdmin).Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	admin := User{
		Username: "admin",
		Password: string(hash),
		Role:     RoleAdmin,
	}

	if err := db.Create(&admin).Error; err != nil {
		return err
	}

	log.Println("[Seed] Created default admin user (username: admin, password: admin123)")
	return nil
}