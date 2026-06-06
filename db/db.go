package db

import (
	"log"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var conn *gorm.DB

// Init opens a SQLite database at path (e.g. "data/regs.db")
func Init(dsn string) error {
	var err error
	conn, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}
	return nil
}

// Get returns the active *gorm.DB connection (may be nil)
func Get() *gorm.DB {
	return conn
}

// MustInit is a helper for quick bootstrapping (logs fatal on error)
func MustInit(dsn string) {
	if err := Init(dsn); err != nil {
		log.Fatalf("failed to open DB: %v", err)
	}
}
