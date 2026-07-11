//go:build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/fsetiawan29/profit-tracker/internal/domain"
	"github.com/fsetiawan29/profit-tracker/internal/repository"
)

// T019: ingredient create/list/update/archive, and cross-shop access denied
// (data-model.md's user_id scoping, FR-002/SC-007 — a repository method
// scoped by userID finds nothing for another shop's row, the same not-found
// outcome the API layer later turns into a 404).
func TestIngredientRepository_CRUDAndCrossShopAccess(t *testing.T) {
	db := setupTestDB(t)

	users := repository.NewUserRepository(db)
	ingredients := repository.NewIngredientRepository(db)

	shopA, err := users.FindOrCreateByTelegramID(1001, nil, nil)
	require.NoError(t, err)
	shopB, err := users.FindOrCreateByTelegramID(1002, nil, nil)
	require.NoError(t, err)

	// Create
	ingredient := &domain.Ingredient{Name: "Flour", UnitOfMeasure: "kg"}
	require.NoError(t, ingredients.Create(shopA.ID, ingredient))
	require.NotZero(t, ingredient.ID)
	require.Equal(t, shopA.ID, ingredient.UserID)
	require.False(t, ingredient.CurrentUnitCost.Valid, "current_unit_cost is null until a first purchase exists")
	require.False(t, ingredient.IsArchived)

	// List — scoped to the owning shop only
	shopAList, err := ingredients.List(shopA.ID, false)
	require.NoError(t, err)
	require.Len(t, shopAList, 1)
	require.Equal(t, "Flour", shopAList[0].Name)

	shopBList, err := ingredients.List(shopB.ID, false)
	require.NoError(t, err)
	require.Empty(t, shopBList, "shop B must not see shop A's ingredients")

	// Update
	updated, err := ingredients.Update(shopA.ID, ingredient.ID, "Wheat Flour", "g")
	require.NoError(t, err)
	require.Equal(t, "Wheat Flour", updated.Name)
	require.Equal(t, "g", updated.UnitOfMeasure)

	// Archive excludes it from the default (non-archived) list, but it
	// remains visible with includeArchived=true.
	archived, err := ingredients.SetArchived(shopA.ID, ingredient.ID, true)
	require.NoError(t, err)
	require.True(t, archived.IsArchived)

	activeOnly, err := ingredients.List(shopA.ID, false)
	require.NoError(t, err)
	require.Empty(t, activeOnly)

	withArchived, err := ingredients.List(shopA.ID, true)
	require.NoError(t, err)
	require.Len(t, withArchived, 1)

	// Unarchive (state transition may toggle back)
	unarchived, err := ingredients.SetArchived(shopA.ID, ingredient.ID, false)
	require.NoError(t, err)
	require.False(t, unarchived.IsArchived)

	// Cross-shop access denied: shop B cannot read, update, or archive
	// shop A's ingredient — every method returns "not found", never the
	// row itself or a different error class.
	_, err = ingredients.Get(shopB.ID, ingredient.ID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	_, err = ingredients.Update(shopB.ID, ingredient.ID, "Hacked Name", "kg")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	_, err = ingredients.SetArchived(shopB.ID, ingredient.ID, true)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	// The denied cross-shop attempts must not have mutated shop A's row.
	untouched, err := ingredients.Get(shopA.ID, ingredient.ID)
	require.NoError(t, err)
	require.Equal(t, "Wheat Flour", untouched.Name)
	require.False(t, untouched.IsArchived)
}
