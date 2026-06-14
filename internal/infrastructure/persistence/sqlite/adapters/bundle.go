package adapters

import "gorm.io/gorm"

type SQLiteBundle struct {
	db *gorm.DB
}

func NewSQLiteBundle(db *gorm.DB) *SQLiteBundle {
	return &SQLiteBundle{db: db}
}
