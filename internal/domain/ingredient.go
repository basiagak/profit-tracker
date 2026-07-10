package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/fsetiawan29/profit-tracker/internal/db"
)

// Ingredient is a raw material purchased and consumed via recipes.
type Ingredient struct {
	ID                       int64          `gorm:"column:id;primaryKey"`
	UserID                   int64          `gorm:"column:user_id;not null;index"`
	Name                     string         `gorm:"column:name;not null"`
	UnitOfMeasure            string         `gorm:"column:unit_of_measure;not null"`
	CurrentUnitCost          db.NullDecimal `gorm:"column:current_unit_cost;type:numeric(14,4)"`
	CurrentUnitCostUpdatedAt *time.Time     `gorm:"column:current_unit_cost_updated_at"`
	IsArchived               bool           `gorm:"column:is_archived;not null;default:false"`
	CreatedAt                time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt                time.Time      `gorm:"column:updated_at;not null"`
}

// TableName overrides GORM's default pluralization to match the migration.
func (Ingredient) TableName() string { return "ingredients" }

// Validate enforces FR-003's required-fields rule.
func (i *Ingredient) Validate() error {
	if strings.TrimSpace(i.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(i.UnitOfMeasure) == "" {
		return errors.New("unit_of_measure is required")
	}
	return nil
}
