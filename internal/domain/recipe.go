package domain

import (
	"errors"
	"time"

	"github.com/fsetiawan29/profit-tracker/internal/db"
)

// ItemIngredient is one line of an item's current recipe: how much of one
// ingredient is needed per unit of the item. Editing this never touches past
// sales — sale_items stores its own independent cost snapshot.
type ItemIngredient struct {
	ID           int64      `gorm:"column:id;primaryKey"`
	ItemID       int64      `gorm:"column:item_id;not null;index"`
	IngredientID int64      `gorm:"column:ingredient_id;not null;index"`
	Quantity     db.Decimal `gorm:"column:quantity;type:numeric(14,4);not null"`
	CreatedAt    time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;not null"`
}

// TableName overrides GORM's default pluralization to match the migration.
func (ItemIngredient) TableName() string { return "item_ingredients" }

// Validate enforces the positive-quantity rule (Edge Cases: zero/negative rejected).
func (r *ItemIngredient) Validate() error {
	if !r.Quantity.IsPositive() {
		return errors.New("quantity must be > 0")
	}
	return nil
}
