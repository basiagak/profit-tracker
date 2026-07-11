package telegram

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/fsetiawan29/profit-tracker/internal/domain"
	"github.com/fsetiawan29/profit-tracker/internal/service"
)

// RegisterCatalogCommands adds /ingredient and /item to b, backed by
// catalog (contracts/telegram-commands.md's Ingredients/Items sections).
//
// Names are taken as single, space-free tokens for every catalog command
// below (e.g. "EspressoShot" rather than "Espresso Shot"); the contract
// explicitly leaves "exact tokenizer choice" as an implementation detail,
// and a quoting/escaping grammar isn't warranted at this scale.
func (b *Bot) RegisterCatalogCommands(catalog *service.CatalogService) {
	b.Register(&Command{
		Name:        "ingredient",
		Usage:       "/ingredient add <name> <unit> | list | archive <name>",
		Description: "Manage your ingredient catalog",
		Handler:     ingredientCommandHandler(catalog),
	})
	b.Register(&Command{
		Name:        "item",
		Usage:       "/item add <name> <price> | list | archive <name> | recipe <item-name> <ingredient-name> <quantity> ...",
		Description: "Manage your menu items and recipes",
		Handler:     itemCommandHandler(catalog),
	})
}

func ingredientCommandHandler(catalog *service.CatalogService) CommandHandler {
	return func(ctx *CommandContext) (string, error) {
		fields := strings.Fields(ctx.Args)
		if len(fields) == 0 {
			return "", fmt.Errorf("usage: /ingredient add <name> <unit> | list | archive <name>")
		}

		switch strings.ToLower(fields[0]) {
		case "add":
			return handleIngredientAdd(catalog, ctx.User.ID, fields[1:])
		case "list":
			return handleIngredientList(catalog, ctx.User.ID)
		case "archive":
			return handleIngredientArchive(catalog, ctx.User.ID, fields[1:])
		default:
			return "", fmt.Errorf("unknown /ingredient subcommand %q", fields[0])
		}
	}
}

func itemCommandHandler(catalog *service.CatalogService) CommandHandler {
	return func(ctx *CommandContext) (string, error) {
		fields := strings.Fields(ctx.Args)
		if len(fields) == 0 {
			return "", fmt.Errorf("usage: /item add <name> <price> | list | archive <name> | recipe <item-name> <ingredient-name> <quantity>")
		}

		switch strings.ToLower(fields[0]) {
		case "add":
			return handleItemAdd(catalog, ctx.User.ID, fields[1:])
		case "list":
			return handleItemList(catalog, ctx.User.ID)
		case "archive":
			return handleItemArchive(catalog, ctx.User.ID, fields[1:])
		case "recipe":
			return handleItemRecipe(catalog, ctx.User.ID, fields[1:])
		default:
			return "", fmt.Errorf("unknown /item subcommand %q", fields[0])
		}
	}
}

func handleIngredientAdd(catalog *service.CatalogService, userID int64, args []string) (string, error) {
	if len(args) != 2 {
		return "", fmt.Errorf("usage: /ingredient add <name> <unit>")
	}
	ingredient, err := catalog.CreateIngredient(userID, args[0], args[1])
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Added ingredient %q (%s).", ingredient.Name, ingredient.UnitOfMeasure), nil
}

func handleIngredientList(catalog *service.CatalogService, userID int64) (string, error) {
	ingredients, err := catalog.ListIngredients(userID, false)
	if err != nil {
		return "", err
	}
	if len(ingredients) == 0 {
		return "No ingredients yet. Add one with /ingredient add <name> <unit>.", nil
	}

	lines := []string{"Ingredients:"}
	for _, ingredient := range ingredients {
		cost := "no cost yet"
		if ingredient.CurrentUnitCost.Valid {
			cost = ingredient.CurrentUnitCost.Decimal.String()
		}
		lines = append(lines, fmt.Sprintf("%s (%s) — %s", ingredient.Name, ingredient.UnitOfMeasure, cost))
	}
	return strings.Join(lines, "\n"), nil
}

func handleIngredientArchive(catalog *service.CatalogService, userID int64, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("usage: /ingredient archive <name>")
	}
	ingredient, err := findIngredientByName(catalog, userID, args[0])
	if err != nil {
		return "", err
	}

	_, warnings, err := catalog.ArchiveIngredient(userID, ingredient.ID, true)
	if err != nil {
		return "", err
	}

	reply := fmt.Sprintf("Archived ingredient %q.", ingredient.Name)
	for _, warning := range warnings {
		reply += "\nWarning: " + warning
	}
	return reply, nil
}

func handleItemAdd(catalog *service.CatalogService, userID int64, args []string) (string, error) {
	if len(args) != 2 {
		return "", fmt.Errorf("usage: /item add <name> <price>")
	}
	price, err := decimal.NewFromString(args[1])
	if err != nil {
		return "", fmt.Errorf("price must be a valid non-negative number")
	}

	item, err := catalog.CreateItem(userID, args[0], price)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Added item %q at %s.", item.Name, item.SalePrice.String()), nil
}

func handleItemList(catalog *service.CatalogService, userID int64) (string, error) {
	items, err := catalog.ListItems(userID, false)
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "No items yet. Add one with /item add <name> <price>.", nil
	}

	lines := []string{"Items:"}
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("%s — %s", item.Name, item.SalePrice.String()))
	}
	return strings.Join(lines, "\n"), nil
}

func handleItemArchive(catalog *service.CatalogService, userID int64, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("usage: /item archive <name>")
	}
	item, err := findItemByName(catalog, userID, args[0])
	if err != nil {
		return "", err
	}

	if _, err := catalog.ArchiveItem(userID, item.ID, true); err != nil {
		return "", err
	}
	return fmt.Sprintf("Archived item %q.", item.Name), nil
}

// handleItemRecipe resolves and validates every (ingredient, quantity) pair
// before writing any of them, so a bad pair partway through the argument
// list rejects the whole command with no partial write.
func handleItemRecipe(catalog *service.CatalogService, userID int64, args []string) (string, error) {
	if len(args) < 3 {
		return "", fmt.Errorf("usage: /item recipe <item-name> <ingredient-name> <quantity> [<ingredient-name> <quantity> ...]")
	}

	item, err := findItemByName(catalog, userID, args[0])
	if err != nil {
		return "", err
	}

	pairs := args[1:]
	if len(pairs)%2 != 0 {
		return "", fmt.Errorf("each ingredient must be followed by a quantity")
	}

	type resolvedLine struct {
		ingredient *domain.Ingredient
		quantity   decimal.Decimal
	}

	resolved := make([]resolvedLine, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		ingredientName := pairs[i]
		quantity, err := decimal.NewFromString(pairs[i+1])
		if err != nil || !quantity.IsPositive() {
			return "", fmt.Errorf("quantity for %q must be a positive number", ingredientName)
		}
		ingredient, err := findIngredientByName(catalog, userID, ingredientName)
		if err != nil {
			return "", err
		}
		resolved = append(resolved, resolvedLine{ingredient: ingredient, quantity: quantity})
	}

	lines := make([]string, 0, len(resolved))
	for _, r := range resolved {
		line, err := catalog.UpsertRecipeLine(userID, item.ID, r.ingredient.ID, r.quantity)
		if err != nil {
			return "", err
		}
		lines = append(lines, fmt.Sprintf("%s: %s %s", line.IngredientName, line.Quantity.String(), r.ingredient.UnitOfMeasure))
	}

	return fmt.Sprintf("Updated recipe for %q:\n%s", item.Name, strings.Join(lines, "\n")), nil
}

func findIngredientByName(catalog *service.CatalogService, userID int64, name string) (*domain.Ingredient, error) {
	ingredients, err := catalog.ListIngredients(userID, true)
	if err != nil {
		return nil, err
	}
	for i := range ingredients {
		if strings.EqualFold(ingredients[i].Name, name) {
			return &ingredients[i], nil
		}
	}
	return nil, fmt.Errorf("no ingredient named %q", name)
}

func findItemByName(catalog *service.CatalogService, userID int64, name string) (*domain.Item, error) {
	items, err := catalog.ListItems(userID, true)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if strings.EqualFold(items[i].Name, name) {
			return &items[i], nil
		}
	}
	return nil, fmt.Errorf("no item named %q", name)
}
