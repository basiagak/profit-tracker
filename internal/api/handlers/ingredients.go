package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/fsetiawan29/profit-tracker/internal/api"
	"github.com/fsetiawan29/profit-tracker/internal/db"
	"github.com/fsetiawan29/profit-tracker/internal/domain"
	"github.com/fsetiawan29/profit-tracker/internal/service"
)

// IngredientsHandler serves the /ingredients routes (contracts/web-api.yaml).
type IngredientsHandler struct {
	catalog *service.CatalogService
}

// NewIngredientsHandler builds an IngredientsHandler.
func NewIngredientsHandler(catalog *service.CatalogService) *IngredientsHandler {
	return &IngredientsHandler{catalog: catalog}
}

// Register mounts GET/POST /ingredients, PATCH /ingredients/:id, and
// POST /ingredients/:id/archive on the session-guarded protected group.
func (h *IngredientsHandler) Register(protected *echo.Group) {
	protected.GET("/ingredients", h.list)
	protected.POST("/ingredients", h.create)
	protected.PATCH("/ingredients/:id", h.update)
	protected.POST("/ingredients/:id/archive", h.archive)
}

type ingredientResponse struct {
	ID              int64          `json:"id"`
	Name            string         `json:"name"`
	UnitOfMeasure   string         `json:"unit_of_measure"`
	CurrentUnitCost db.NullDecimal `json:"current_unit_cost"`
	IsArchived      bool           `json:"is_archived"`
}

func newIngredientResponse(i *domain.Ingredient) ingredientResponse {
	return ingredientResponse{
		ID:              i.ID,
		Name:            i.Name,
		UnitOfMeasure:   i.UnitOfMeasure,
		CurrentUnitCost: i.CurrentUnitCost,
		IsArchived:      i.IsArchived,
	}
}

type ingredientArchiveResponse struct {
	ingredientResponse
	Warnings []string `json:"warnings,omitempty"`
}

func (h *IngredientsHandler) list(c echo.Context) error {
	userID, _ := api.UserIDFromContext(c)
	includeArchived, _ := strconv.ParseBool(c.QueryParam("include_archived"))

	ingredients, err := h.catalog.ListIngredients(userID, includeArchived)
	if err != nil {
		return mapCatalogError(err)
	}

	responses := make([]ingredientResponse, len(ingredients))
	for i := range ingredients {
		responses[i] = newIngredientResponse(&ingredients[i])
	}
	return c.JSON(http.StatusOK, responses)
}

func (h *IngredientsHandler) create(c echo.Context) error {
	userID, _ := api.UserIDFromContext(c)

	var req struct {
		Name          string `json:"name"`
		UnitOfMeasure string `json:"unit_of_measure"`
	}
	if err := c.Bind(&req); err != nil {
		return api.NewAPIError(http.StatusBadRequest, "invalid_request", "malformed request body")
	}

	ingredient, err := h.catalog.CreateIngredient(userID, req.Name, req.UnitOfMeasure)
	if err != nil {
		return mapCatalogError(err)
	}
	return c.JSON(http.StatusCreated, newIngredientResponse(ingredient))
}

func (h *IngredientsHandler) update(c echo.Context) error {
	userID, _ := api.UserIDFromContext(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return api.NewAPIError(http.StatusNotFound, "not_found", "ingredient not found")
	}

	current, err := h.catalog.GetIngredient(userID, id)
	if err != nil {
		return mapCatalogError(err)
	}

	var req struct {
		Name          *string `json:"name"`
		UnitOfMeasure *string `json:"unit_of_measure"`
	}
	if err := c.Bind(&req); err != nil {
		return api.NewAPIError(http.StatusBadRequest, "invalid_request", "malformed request body")
	}

	name := current.Name
	if req.Name != nil {
		name = *req.Name
	}
	unitOfMeasure := current.UnitOfMeasure
	if req.UnitOfMeasure != nil {
		unitOfMeasure = *req.UnitOfMeasure
	}

	updated, err := h.catalog.UpdateIngredient(userID, id, name, unitOfMeasure)
	if err != nil {
		return mapCatalogError(err)
	}
	return c.JSON(http.StatusOK, newIngredientResponse(updated))
}

func (h *IngredientsHandler) archive(c echo.Context) error {
	userID, _ := api.UserIDFromContext(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return api.NewAPIError(http.StatusNotFound, "not_found", "ingredient not found")
	}

	var req struct {
		Archived bool `json:"archived"`
	}
	if err := c.Bind(&req); err != nil {
		return api.NewAPIError(http.StatusBadRequest, "invalid_request", "malformed request body")
	}

	ingredient, warnings, err := h.catalog.ArchiveIngredient(userID, id, req.Archived)
	if err != nil {
		return mapCatalogError(err)
	}

	return c.JSON(http.StatusOK, ingredientArchiveResponse{
		ingredientResponse: newIngredientResponse(ingredient),
		Warnings:           warnings,
	})
}
