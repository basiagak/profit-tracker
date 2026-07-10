package domain

import (
	"errors"
	"time"

	"github.com/fsetiawan29/profit-tracker/internal/db"
)

// IngredientPurchase is an immutable record of buying a quantity of an
// ingredient for a total cost on a date. Rows are never updated after insert
// (FR-012).
type IngredientPurchase struct {
	ID           int64      `gorm:"column:id;primaryKey"`
	UserID       int64      `gorm:"column:user_id;not null;index"`
	IngredientID int64      `gorm:"column:ingredient_id;not null;index"`
	Quantity     db.Decimal `gorm:"column:quantity;type:numeric(14,4);not null"`
	TotalCost    db.Decimal `gorm:"column:total_cost;type:numeric(14,4);not null"`
	UnitCost     db.Decimal `gorm:"column:unit_cost;type:numeric(14,4);not null"`
	PurchaseDate time.Time  `gorm:"column:purchase_date;not null"`
	Notes        *string    `gorm:"column:notes"`
	CreatedAt    time.Time  `gorm:"column:created_at;not null"`
}

// TableName overrides GORM's default pluralization to match the migration.
func (IngredientPurchase) TableName() string { return "ingredient_purchases" }

// Validate enforces FR-009 / Edge Cases: positive quantity, non-negative cost.
func (p *IngredientPurchase) Validate() error {
	if !p.Quantity.IsPositive() {
		return errors.New("quantity must be > 0")
	}
	if p.TotalCost.IsNegative() {
		return errors.New("total_cost must be >= 0")
	}
	return nil
}
