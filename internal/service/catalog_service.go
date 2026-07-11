// Package service holds the business-logic layer between repositories and
// the Telegram/API delivery surfaces, so both surfaces share one set of
// validation and orchestration rules.
package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"

	"github.com/fsetiawan29/profit-tracker/internal/db"
	"github.com/fsetiawan29/profit-tracker/internal/domain"
	"github.com/fsetiawan29/profit-tracker/internal/repository"
)

// postgresUniqueViolation is the SQLSTATE Postgres reports for a unique
// constraint violation (data-model.md's (user_id, lower(name)) index).
const postgresUniqueViolation = "23505"

// ErrDuplicateName is returned when creating or renaming an ingredient or
// item would collide with another active row of the same name in the same
// shop (data-model.md's unique (user_id, lower(name)) constraint).
var ErrDuplicateName = errors.New("an active ingredient or item with this name already exists")

// ValidationError wraps a catalog rule violation (required fields,
// quantity/price bounds) so delivery surfaces can distinguish it from
// not-found/duplicate/internal errors (e.g. to answer 422 vs 404 vs 409).
type ValidationError struct {
	msg string
}

func (e *ValidationError) Error() string { return e.msg }

func newValidationError(err error) *ValidationError {
	return &ValidationError{msg: err.Error()}
}

// RecipeLine is one line of an item's current recipe, enriched with the
// ingredient's name for display (contracts/web-api.yaml's RecipeLine
// schema; contracts/telegram-commands.md's /item recipe reply).
type RecipeLine struct {
	IngredientID   int64
	IngredientName string
	Quantity       db.Decimal
}

// CatalogService orchestrates ingredient/item/recipe validation on top of
// the repository layer: required-field and quantity/price rules, duplicate
// active-name detection (409), cross-tenant recipe reference rejection, and
// the archive-while-referenced warning (data-model.md Edge Cases).
type CatalogService struct {
	ingredients *repository.IngredientRepository
	items       *repository.ItemRepository
}

// NewCatalogService builds a CatalogService.
func NewCatalogService(ingredients *repository.IngredientRepository, items *repository.ItemRepository) *CatalogService {
	return &CatalogService{ingredients: ingredients, items: items}
}

// --- Ingredients ---

// CreateIngredient validates and creates an ingredient (FR-003).
func (s *CatalogService) CreateIngredient(userID int64, name, unitOfMeasure string) (*domain.Ingredient, error) {
	ingredient := &domain.Ingredient{Name: name, UnitOfMeasure: unitOfMeasure}
	if err := ingredient.Validate(); err != nil {
		return nil, newValidationError(err)
	}
	if err := s.ingredients.Create(userID, ingredient); err != nil {
		return nil, translateDuplicate(err)
	}
	return ingredient, nil
}

// ListIngredients returns userID's ingredients (FR-005 selection lists).
func (s *CatalogService) ListIngredients(userID int64, includeArchived bool) ([]domain.Ingredient, error) {
	return s.ingredients.List(userID, includeArchived)
}

// GetIngredient returns one ingredient owned by userID.
func (s *CatalogService) GetIngredient(userID, id int64) (*domain.Ingredient, error) {
	return s.ingredients.Get(userID, id)
}

// UpdateIngredient validates and updates an ingredient's name/unit (FR-003).
func (s *CatalogService) UpdateIngredient(userID, id int64, name, unitOfMeasure string) (*domain.Ingredient, error) {
	candidate := &domain.Ingredient{Name: name, UnitOfMeasure: unitOfMeasure}
	if err := candidate.Validate(); err != nil {
		return nil, newValidationError(err)
	}

	ingredient, err := s.ingredients.Update(userID, id, name, unitOfMeasure)
	if err != nil {
		return nil, translateDuplicate(err)
	}
	return ingredient, nil
}

// ArchiveIngredient toggles is_archived (FR-005). Archiving is always
// allowed, even when the ingredient is still referenced by an active item's
// recipe (data-model.md Edge Cases) — in that case the referencing items'
// names are returned as warnings, not an error.
func (s *CatalogService) ArchiveIngredient(userID, id int64, archived bool) (*domain.Ingredient, []string, error) {
	ingredient, err := s.ingredients.SetArchived(userID, id, archived)
	if err != nil {
		return nil, nil, err
	}

	var warnings []string
	if archived {
		referencing, err := s.items.ListActiveByIngredient(userID, id)
		if err != nil {
			return nil, nil, err
		}
		if len(referencing) > 0 {
			names := make([]string, len(referencing))
			for i, item := range referencing {
				names[i] = item.Name
			}
			warnings = append(warnings, fmt.Sprintf("still referenced by active item(s): %s", strings.Join(names, ", ")))
		}
	}
	return ingredient, warnings, nil
}

// --- Items ---

// CreateItem validates and creates a menu item (FR-004).
func (s *CatalogService) CreateItem(userID int64, name string, salePrice decimal.Decimal) (*domain.Item, error) {
	item := &domain.Item{Name: name, SalePrice: db.NewDecimal(salePrice)}
	if err := item.Validate(); err != nil {
		return nil, newValidationError(err)
	}
	if err := s.items.Create(userID, item); err != nil {
		return nil, translateDuplicate(err)
	}
	return item, nil
}

// ListItems returns userID's items (FR-005 selection lists).
func (s *CatalogService) ListItems(userID int64, includeArchived bool) ([]domain.Item, error) {
	return s.items.List(userID, includeArchived)
}

// GetItem returns one item owned by userID.
func (s *CatalogService) GetItem(userID, id int64) (*domain.Item, error) {
	return s.items.Get(userID, id)
}

// UpdateItem validates and updates an item's name/price (FR-004).
func (s *CatalogService) UpdateItem(userID, id int64, name string, salePrice decimal.Decimal) (*domain.Item, error) {
	candidate := &domain.Item{Name: name, SalePrice: db.NewDecimal(salePrice)}
	if err := candidate.Validate(); err != nil {
		return nil, newValidationError(err)
	}

	item, err := s.items.Update(userID, id, name, db.NewDecimal(salePrice))
	if err != nil {
		return nil, translateDuplicate(err)
	}
	return item, nil
}

// ArchiveItem toggles is_archived on an item (FR-005). Unlike ingredients,
// items have no "referenced by" relationship in this domain, so no warning
// is ever produced.
func (s *CatalogService) ArchiveItem(userID, id int64, archived bool) (*domain.Item, error) {
	return s.items.SetArchived(userID, id, archived)
}

// --- Recipe ---

// ListRecipe returns itemID's current recipe, enriched with ingredient
// names (FR-006).
func (s *CatalogService) ListRecipe(userID, itemID int64) ([]RecipeLine, error) {
	lines, err := s.items.ListRecipe(userID, itemID)
	if err != nil {
		return nil, err
	}

	result := make([]RecipeLine, 0, len(lines))
	for _, line := range lines {
		ingredient, err := s.ingredients.Get(userID, line.IngredientID)
		if err != nil {
			return nil, err
		}
		result = append(result, RecipeLine{
			IngredientID:   ingredient.ID,
			IngredientName: ingredient.Name,
			Quantity:       line.Quantity,
		})
	}
	return result, nil
}

// UpsertRecipeLine sets the quantity for (itemID, ingredientID) — creating
// or updating the line in place (FR-006, FR-007). It validates quantity > 0
// and that ingredientID belongs to userID's shop, rejecting cross-tenant
// recipe references even though the foreign key itself would allow them
// (data-model.md: "item_id and ingredient_id must belong to the same
// user_id ... enforced in the service layer, not just FK").
func (s *CatalogService) UpsertRecipeLine(userID, itemID, ingredientID int64, quantity decimal.Decimal) (*RecipeLine, error) {
	candidate := &domain.ItemIngredient{Quantity: db.NewDecimal(quantity)}
	if err := candidate.Validate(); err != nil {
		return nil, newValidationError(err)
	}

	ingredient, err := s.ingredients.Get(userID, ingredientID)
	if err != nil {
		return nil, err
	}

	line, err := s.items.UpsertRecipeLine(userID, itemID, ingredientID, db.NewDecimal(quantity))
	if err != nil {
		return nil, err
	}

	return &RecipeLine{IngredientID: ingredient.ID, IngredientName: ingredient.Name, Quantity: line.Quantity}, nil
}

// DeleteRecipeLine removes one recipe line (after confirming itemID belongs
// to userID, via the repository).
func (s *CatalogService) DeleteRecipeLine(userID, itemID, ingredientID int64) error {
	return s.items.DeleteRecipeLine(userID, itemID, ingredientID)
}

// translateDuplicate converts a Postgres unique-violation on the
// (user_id, lower(name)) index into ErrDuplicateName, passing through any
// other error unchanged.
func translateDuplicate(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == postgresUniqueViolation {
		return ErrDuplicateName
	}
	return err
}
