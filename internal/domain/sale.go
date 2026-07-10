package domain

import (
	"errors"
	"time"

	"github.com/fsetiawan29/profit-tracker/internal/db"
)

// SaleSource identifies which surface recorded a sale (FR-013).
type SaleSource string

const (
	SaleSourceTelegram  SaleSource = "telegram"
	SaleSourceDashboard SaleSource = "dashboard"
)

// Sale is a single sale transaction, composed of one or more SaleItem lines.
type Sale struct {
	ID        int64      `gorm:"column:id;primaryKey"`
	UserID    int64      `gorm:"column:user_id;not null;index"`
	SoldAt    time.Time  `gorm:"column:sold_at;not null"`
	Source    SaleSource `gorm:"column:source;not null"`
	CreatedAt time.Time  `gorm:"column:created_at;not null"`
	Items     []SaleItem `gorm:"foreignKey:SaleID"`
}

// TableName overrides GORM's default pluralization to match the migration.
func (Sale) TableName() string { return "sales" }

// SaleItem is one line within a sale: quantity of one item sold, with its
// production cost and price permanently snapshotted at the moment of sale
// (FR-014, FR-015).
type SaleItem struct {
	ID                 int64      `gorm:"column:id;primaryKey"`
	SaleID             int64      `gorm:"column:sale_id;not null;index"`
	ItemID             int64      `gorm:"column:item_id;not null;index"`
	Quantity           db.Decimal `gorm:"column:quantity;type:numeric(14,4);not null"`
	UnitPrice          db.Decimal `gorm:"column:unit_price;type:numeric(12,2);not null"`
	UnitProductionCost db.Decimal `gorm:"column:unit_production_cost;type:numeric(14,4);not null"`
	CreatedAt          time.Time  `gorm:"column:created_at;not null"`
}

// TableName overrides GORM's default pluralization to match the migration.
func (SaleItem) TableName() string { return "sale_items" }

// Validate enforces the positive-quantity rule for a sale line.
func (si *SaleItem) Validate() error {
	if !si.Quantity.IsPositive() {
		return errors.New("quantity must be > 0")
	}
	return nil
}
