package models

import "github.com/jinzhu/gorm"

// Gallery represents the galleries table in our DB
// and is mostly a container resource composed of images.
type Gallery struct {
	gorm.Model
	UserID uint   `gorm:"not_null;index"`
	Title  string `gorm:"not_null"`
}
