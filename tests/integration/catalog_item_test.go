//go:build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fsetiawan29/profit-tracker/internal/db"
	"github.com/fsetiawan29/profit-tracker/internal/domain"
	"github.com/fsetiawan29/profit-tracker/internal/repository"
)

// T020: item create/list/update/archive, recipe upsert/list/delete, and the
// archive-while-referenced edge case (data-model.md Edge Cases: archiving an
// ingredient still referenced by an active recipe is allowed — it only
// affects selection lists, never the existing item_ingredients row. Turning
// that into an actual user-facing warning is internal/service/catalog_service.go's
// job (T023); this repository-level test only asserts the underlying
// invariant the warning depends on: archiving succeeds and the recipe keeps
// resolving).
func TestItemRepository_CRUDRecipeAndArchiveWhileReferenced(t *testing.T) {
	gdb := setupTestDB(t)

	users := repository.NewUserRepository(gdb)
	ingredients := repository.NewIngredientRepository(gdb)
	items := repository.NewItemRepository(gdb)

	shop, err := users.FindOrCreateByTelegramID(2001, nil, nil)
	require.NoError(t, err)

	salePrice, err := db.NewDecimalFromString("9.50")
	require.NoError(t, err)

	// Create
	item := &domain.Item{Name: "Cappuccino", SalePrice: salePrice}
	require.NoError(t, items.Create(shop.ID, item))
	require.NotZero(t, item.ID)
	require.Equal(t, shop.ID, item.UserID)
	require.False(t, item.IsArchived)

	// List / Get
	list, err := items.List(shop.ID, false)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "Cappuccino", list[0].Name)

	fetched, err := items.Get(shop.ID, item.ID)
	require.NoError(t, err)
	require.Equal(t, "Cappuccino", fetched.Name)

	// Update
	newPrice, err := db.NewDecimalFromString("10.00")
	require.NoError(t, err)
	updated, err := items.Update(shop.ID, item.ID, "Large Cappuccino", newPrice)
	require.NoError(t, err)
	require.Equal(t, "Large Cappuccino", updated.Name)
	require.True(t, updated.SalePrice.Equal(newPrice.Decimal))

	// Archive / unarchive
	archived, err := items.SetArchived(shop.ID, item.ID, true)
	require.NoError(t, err)
	require.True(t, archived.IsArchived)

	activeOnly, err := items.List(shop.ID, false)
	require.NoError(t, err)
	require.Empty(t, activeOnly)

	unarchived, err := items.SetArchived(shop.ID, item.ID, false)
	require.NoError(t, err)
	require.False(t, unarchived.IsArchived)

	// Recipe: upsert, list, delete
	ingredient := &domain.Ingredient{Name: "Espresso Shot", UnitOfMeasure: "shot"}
	require.NoError(t, ingredients.Create(shop.ID, ingredient))

	qty, err := db.NewDecimalFromString("2")
	require.NoError(t, err)
	line, err := items.UpsertRecipeLine(shop.ID, item.ID, ingredient.ID, qty)
	require.NoError(t, err)
	require.True(t, line.Quantity.Equal(qty.Decimal))

	recipe, err := items.ListRecipe(shop.ID, item.ID)
	require.NoError(t, err)
	require.Len(t, recipe, 1)
	require.Equal(t, ingredient.ID, recipe[0].IngredientID)

	// Upserting the same (item, ingredient) pair updates the existing row in
	// place rather than inserting a second one (data-model.md: "editing a
	// quantity updates the existing row in place").
	newQty, err := db.NewDecimalFromString("3")
	require.NoError(t, err)
	updatedLine, err := items.UpsertRecipeLine(shop.ID, item.ID, ingredient.ID, newQty)
	require.NoError(t, err)
	require.Equal(t, line.ID, updatedLine.ID)
	require.True(t, updatedLine.Quantity.Equal(newQty.Decimal))

	recipeAfterUpsert, err := items.ListRecipe(shop.ID, item.ID)
	require.NoError(t, err)
	require.Len(t, recipeAfterUpsert, 1)

	// Archiving an ingredient still referenced by this active item's recipe
	// must succeed (not be blocked by the FK) and must not touch the
	// existing item_ingredients row.
	_, err = ingredients.SetArchived(shop.ID, ingredient.ID, true)
	require.NoError(t, err)

	recipeAfterIngredientArchive, err := items.ListRecipe(shop.ID, item.ID)
	require.NoError(t, err)
	require.Len(t, recipeAfterIngredientArchive, 1, "archiving the ingredient must not remove the existing recipe line")
	require.True(t, recipeAfterIngredientArchive[0].Quantity.Equal(newQty.Decimal))

	require.NoError(t, items.DeleteRecipeLine(shop.ID, item.ID, ingredient.ID))

	recipeAfterDelete, err := items.ListRecipe(shop.ID, item.ID)
	require.NoError(t, err)
	require.Empty(t, recipeAfterDelete)
}
