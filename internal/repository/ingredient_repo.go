package repository

import (
	"gorm.io/gorm"

	"github.com/fsetiawan29/profit-tracker/internal/domain"
)

// IngredientRepository persists and looks up a shop's raw materials. Every
// method is scoped by userID (research.md §8) — there is no method that
// reads/writes an ingredient without it.
type IngredientRepository struct {
	db *gorm.DB
}

// NewIngredientRepository builds an IngredientRepository over the given GORM handle.
func NewIngredientRepository(db *gorm.DB) *IngredientRepository {
	return &IngredientRepository{db: db}
}

// Create inserts ingredient under userID's shop.
func (r *IngredientRepository) Create(userID int64, ingredient *domain.Ingredient) error {
	ingredient.UserID = userID
	return r.db.Create(ingredient).Error
}

// List returns userID's ingredients, ordered by id. Archived ingredients are
// included only when includeArchived is true (FR-005).
func (r *IngredientRepository) List(userID int64, includeArchived bool) ([]domain.Ingredient, error) {
	query := r.db.Where("user_id = ?", userID)
	if !includeArchived {
		query = query.Where("is_archived = false")
	}

	var ingredients []domain.Ingredient
	if err := query.Order("id").Find(&ingredients).Error; err != nil {
		return nil, err
	}
	return ingredients, nil
}

// Get returns the ingredient with id owned by userID. It returns
// gorm.ErrRecordNotFound if the ingredient doesn't exist or belongs to
// another shop (FR-002, SC-007 — cross-shop access is indistinguishable
// from not found).
func (r *IngredientRepository) Get(userID, id int64) (*domain.Ingredient, error) {
	var ingredient domain.Ingredient
	if err := r.db.Where("user_id = ? AND id = ?", userID, id).First(&ingredient).Error; err != nil {
		return nil, err
	}
	return &ingredient, nil
}

// Update sets name/unitOfMeasure on the ingredient owned by userID (FR-003).
// Returns gorm.ErrRecordNotFound for a cross-shop id.
func (r *IngredientRepository) Update(userID, id int64, name, unitOfMeasure string) (*domain.Ingredient, error) {
	ingredient, err := r.Get(userID, id)
	if err != nil {
		return nil, err
	}

	ingredient.Name = name
	ingredient.UnitOfMeasure = unitOfMeasure
	if err := r.db.Save(ingredient).Error; err != nil {
		return nil, err
	}
	return ingredient, nil
}

// SetArchived toggles is_archived on the ingredient owned by userID (FR-005).
// Returns gorm.ErrRecordNotFound for a cross-shop id.
func (r *IngredientRepository) SetArchived(userID, id int64, archived bool) (*domain.Ingredient, error) {
	ingredient, err := r.Get(userID, id)
	if err != nil {
		return nil, err
	}

	ingredient.IsArchived = archived
	if err := r.db.Save(ingredient).Error; err != nil {
		return nil, err
	}
	return ingredient, nil
}
