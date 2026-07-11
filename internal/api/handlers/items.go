package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"

	"github.com/fsetiawan29/profit-tracker/internal/api"
	"github.com/fsetiawan29/profit-tracker/internal/db"
	"github.com/fsetiawan29/profit-tracker/internal/domain"
	"github.com/fsetiawan29/profit-tracker/internal/service"
)

// ItemsHandler serves the /items and /items/{id}/recipe routes
// (contracts/web-api.yaml).
type ItemsHandler struct {
	catalog *service.CatalogService
}

// NewItemsHandler builds an ItemsHandler.
func NewItemsHandler(catalog *service.CatalogService) *ItemsHandler {
	return &ItemsHandler{catalog: catalog}
}

// Register mounts the item CRUD routes plus the recipe sub-resource routes
// on the session-guarded protected group.
func (h *ItemsHandler) Register(protected *echo.Group) {
	protected.GET("/items", h.list)
	protected.POST("/items", h.create)
	protected.PATCH("/items/:id", h.update)
	protected.POST("/items/:id/archive", h.archive)
	protected.GET("/items/:id/recipe", h.getRecipe)
	protected.PUT("/items/:id/recipe/:ingredient_id", h.upsertRecipeLine)
	protected.DELETE("/items/:id/recipe/:ingredient_id", h.deleteRecipeLine)
}

type itemResponse struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	SalePrice  db.Decimal `json:"sale_price"`
	IsArchived bool       `json:"is_archived"`
}

func newItemResponse(i *domain.Item) itemResponse {
	return itemResponse{ID: i.ID, Name: i.Name, SalePrice: i.SalePrice, IsArchived: i.IsArchived}
}

type recipeLineResponse struct {
	IngredientID   int64      `json:"ingredient_id"`
	IngredientName string     `json:"ingredient_name"`
	Quantity       db.Decimal `json:"quantity"`
}

func newRecipeLineResponse(l service.RecipeLine) recipeLineResponse {
	return recipeLineResponse{
		IngredientID:   l.IngredientID,
		IngredientName: l.IngredientName,
		Quantity:       l.Quantity,
	}
}

func (h *ItemsHandler) list(c echo.Context) error {
	userID, _ := api.UserIDFromContext(c)
	includeArchived, _ := strconv.ParseBool(c.QueryParam("include_archived"))

	items, err := h.catalog.ListItems(userID, includeArchived)
	if err != nil {
		return mapCatalogError(err)
	}

	responses := make([]itemResponse, len(items))
	for i := range items {
		responses[i] = newItemResponse(&items[i])
	}
	return c.JSON(http.StatusOK, responses)
}

func (h *ItemsHandler) create(c echo.Context) error {
	userID, _ := api.UserIDFromContext(c)

	var req struct {
		Name      string `json:"name"`
		SalePrice string `json:"sale_price"`
	}
	if err := c.Bind(&req); err != nil {
		return api.NewAPIError(http.StatusBadRequest, "invalid_request", "malformed request body")
	}

	price, err := decimal.NewFromString(req.SalePrice)
	if err != nil {
		return api.NewAPIError(http.StatusUnprocessableEntity, "invalid_request", "sale_price must be a decimal string")
	}

	item, err := h.catalog.CreateItem(userID, req.Name, price)
	if err != nil {
		return mapCatalogError(err)
	}
	return c.JSON(http.StatusCreated, newItemResponse(item))
}

func (h *ItemsHandler) update(c echo.Context) error {
	userID, _ := api.UserIDFromContext(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return api.NewAPIError(http.StatusNotFound, "not_found", "item not found")
	}

	current, err := h.catalog.GetItem(userID, id)
	if err != nil {
		return mapCatalogError(err)
	}

	var req struct {
		Name      *string `json:"name"`
		SalePrice *string `json:"sale_price"`
	}
	if err := c.Bind(&req); err != nil {
		return api.NewAPIError(http.StatusBadRequest, "invalid_request", "malformed request body")
	}

	name := current.Name
	if req.Name != nil {
		name = *req.Name
	}
	price := current.SalePrice.Decimal
	if req.SalePrice != nil {
		parsed, err := decimal.NewFromString(*req.SalePrice)
		if err != nil {
			return api.NewAPIError(http.StatusUnprocessableEntity, "invalid_request", "sale_price must be a decimal string")
		}
		price = parsed
	}

	updated, err := h.catalog.UpdateItem(userID, id, name, price)
	if err != nil {
		return mapCatalogError(err)
	}
	return c.JSON(http.StatusOK, newItemResponse(updated))
}

func (h *ItemsHandler) archive(c echo.Context) error {
	userID, _ := api.UserIDFromContext(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return api.NewAPIError(http.StatusNotFound, "not_found", "item not found")
	}

	var req struct {
		Archived bool `json:"archived"`
	}
	if err := c.Bind(&req); err != nil {
		return api.NewAPIError(http.StatusBadRequest, "invalid_request", "malformed request body")
	}

	item, err := h.catalog.ArchiveItem(userID, id, req.Archived)
	if err != nil {
		return mapCatalogError(err)
	}
	return c.JSON(http.StatusOK, newItemResponse(item))
}

func (h *ItemsHandler) getRecipe(c echo.Context) error {
	userID, _ := api.UserIDFromContext(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return api.NewAPIError(http.StatusNotFound, "not_found", "item not found")
	}

	lines, err := h.catalog.ListRecipe(userID, id)
	if err != nil {
		return mapCatalogError(err)
	}

	responses := make([]recipeLineResponse, len(lines))
	for i, line := range lines {
		responses[i] = newRecipeLineResponse(line)
	}
	return c.JSON(http.StatusOK, responses)
}

func (h *ItemsHandler) upsertRecipeLine(c echo.Context) error {
	userID, _ := api.UserIDFromContext(c)
	itemID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return api.NewAPIError(http.StatusNotFound, "not_found", "item not found")
	}
	ingredientID, err := strconv.ParseInt(c.Param("ingredient_id"), 10, 64)
	if err != nil {
		return api.NewAPIError(http.StatusNotFound, "not_found", "ingredient not found")
	}

	var req struct {
		Quantity string `json:"quantity"`
	}
	if err := c.Bind(&req); err != nil {
		return api.NewAPIError(http.StatusBadRequest, "invalid_request", "malformed request body")
	}

	quantity, err := decimal.NewFromString(req.Quantity)
	if err != nil {
		return api.NewAPIError(http.StatusUnprocessableEntity, "invalid_request", "quantity must be a decimal string")
	}

	line, err := h.catalog.UpsertRecipeLine(userID, itemID, ingredientID, quantity)
	if err != nil {
		return mapCatalogError(err)
	}
	return c.JSON(http.StatusOK, newRecipeLineResponse(*line))
}

func (h *ItemsHandler) deleteRecipeLine(c echo.Context) error {
	userID, _ := api.UserIDFromContext(c)
	itemID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return api.NewAPIError(http.StatusNotFound, "not_found", "item not found")
	}
	ingredientID, err := strconv.ParseInt(c.Param("ingredient_id"), 10, 64)
	if err != nil {
		return api.NewAPIError(http.StatusNotFound, "not_found", "ingredient not found")
	}

	if err := h.catalog.DeleteRecipeLine(userID, itemID, ingredientID); err != nil {
		return mapCatalogError(err)
	}
	return c.NoContent(http.StatusNoContent)
}
