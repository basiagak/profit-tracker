package repository

import (
	"errors"

	"gorm.io/gorm"

	"github.com/fsetiawan29/profit-tracker/internal/db"
	"github.com/fsetiawan29/profit-tracker/internal/domain"
)

// ItemRepository persists and looks up a shop's menu items and their
// recipes (item_ingredients). Every method is scoped by userID (research.md
// §8) — there is no method that reads/writes an item without it.
type ItemRepository struct {
	db *gorm.DB
}

// NewItemRepository builds an ItemRepository over the given GORM handle.
func NewItemRepository(db *gorm.DB) *ItemRepository {
	return &ItemRepository{db: db}
}

// Create inserts item under userID's shop.
func (r *ItemRepository) Create(userID int64, item *domain.Item) error {
	item.UserID = userID
	return r.db.Create(item).Error
}

// List returns userID's items, ordered by id. Archived items are included
// only when includeArchived is true (FR-005).
func (r *ItemRepository) List(userID int64, includeArchived bool) ([]domain.Item, error) {
	query := r.db.Where("user_id = ?", userID)
	if !includeArchived {
		query = query.Where("is_archived = false")
	}

	var items []domain.Item
	if err := query.Order("id").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// Get returns the item with id owned by userID. It returns
// gorm.ErrRecordNotFound if the item doesn't exist or belongs to another
// shop (FR-002, SC-007).
func (r *ItemRepository) Get(userID, id int64) (*domain.Item, error) {
	var item domain.Item
	if err := r.db.Where("user_id = ? AND id = ?", userID, id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// Update sets name/salePrice on the item owned by userID (FR-004). Returns
// gorm.ErrRecordNotFound for a cross-shop id.
func (r *ItemRepository) Update(userID, id int64, name string, salePrice db.Decimal) (*domain.Item, error) {
	item, err := r.Get(userID, id)
	if err != nil {
		return nil, err
	}

	item.Name = name
	item.SalePrice = salePrice
	if err := r.db.Save(item).Error; err != nil {
		return nil, err
	}
	return item, nil
}

// SetArchived toggles is_archived on the item owned by userID (FR-005).
// Returns gorm.ErrRecordNotFound for a cross-shop id.
func (r *ItemRepository) SetArchived(userID, id int64, archived bool) (*domain.Item, error) {
	item, err := r.Get(userID, id)
	if err != nil {
		return nil, err
	}

	item.IsArchived = archived
	if err := r.db.Save(item).Error; err != nil {
		return nil, err
	}
	return item, nil
}

// UpsertRecipeLine sets quantity for the (itemID, ingredientID) pair, after
// confirming itemID belongs to userID. It updates the existing row in place
// if one already exists for that pair, otherwise inserts a new one
// (data-model.md: "editing a quantity updates the existing row in place").
func (r *ItemRepository) UpsertRecipeLine(userID, itemID, ingredientID int64, quantity db.Decimal) (*domain.ItemIngredient, error) {
	if _, err := r.Get(userID, itemID); err != nil {
		return nil, err
	}

	var line domain.ItemIngredient
	err := r.db.Where("item_id = ? AND ingredient_id = ?", itemID, ingredientID).First(&line).Error
	switch {
	case err == nil:
		line.Quantity = quantity
		if err := r.db.Save(&line).Error; err != nil {
			return nil, err
		}
		return &line, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		line = domain.ItemIngredient{ItemID: itemID, IngredientID: ingredientID, Quantity: quantity}
		if err := r.db.Create(&line).Error; err != nil {
			return nil, err
		}
		return &line, nil
	default:
		return nil, err
	}
}

// ListRecipe returns itemID's current recipe lines, after confirming itemID
// belongs to userID.
func (r *ItemRepository) ListRecipe(userID, itemID int64) ([]domain.ItemIngredient, error) {
	if _, err := r.Get(userID, itemID); err != nil {
		return nil, err
	}

	var lines []domain.ItemIngredient
	if err := r.db.Where("item_id = ?", itemID).Order("id").Find(&lines).Error; err != nil {
		return nil, err
	}
	return lines, nil
}

// DeleteRecipeLine removes the (itemID, ingredientID) recipe line, after
// confirming itemID belongs to userID. Deleting a line that doesn't exist is
// a no-op, not an error.
func (r *ItemRepository) DeleteRecipeLine(userID, itemID, ingredientID int64) error {
	if _, err := r.Get(userID, itemID); err != nil {
		return err
	}
	return r.db.Where("item_id = ? AND ingredient_id = ?", itemID, ingredientID).Delete(&domain.ItemIngredient{}).Error
}

// ListActiveByIngredient returns userID's non-archived items whose current
// recipe references ingredientID. Used to build the archive-while-referenced
// warning (data-model.md Edge Cases) without blocking the archive itself.
func (r *ItemRepository) ListActiveByIngredient(userID, ingredientID int64) ([]domain.Item, error) {
	var items []domain.Item
	err := r.db.
		Joins("JOIN item_ingredients ON item_ingredients.item_id = items.id").
		Where("items.user_id = ? AND items.is_archived = false AND item_ingredients.ingredient_id = ?", userID, ingredientID).
		Order("items.id").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}
