package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/fsetiawan29/profit-tracker/internal/db"
)

// Item is a menu item sold to customers.
type Item struct {
	ID         int64      `gorm:"column:id;primaryKey"`
	UserID     int64      `gorm:"column:user_id;not null;index"`
	Name       string     `gorm:"column:name;not null"`
	SalePrice  db.Decimal `gorm:"column:sale_price;type:numeric(12,2);not null"`
	IsArchived bool       `gorm:"column:is_archived;not null;default:false"`
	CreatedAt  time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt  time.Time  `gorm:"column:updated_at;not null"`
}

// TableName overrides GORM's default pluralization to match the migration.
func (Item) TableName() string { return "items" }

// Validate enforces FR-004's required-fields and non-negative-price rules.
func (i *Item) Validate() error {
	if strings.TrimSpace(i.Name) == "" {
		return errors.New("name is required")
	}
	if i.SalePrice.IsNegative() {
		return errors.New("sale_price must be >= 0")
	}
	return nil
}
