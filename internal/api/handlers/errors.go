package handlers

import (
	"errors"
	"net/http"

	"gorm.io/gorm"

	"github.com/fsetiawan29/profit-tracker/internal/api"
	"github.com/fsetiawan29/profit-tracker/internal/service"
)

// mapCatalogError translates CatalogService/repository errors into the
// APIError shape contracts/web-api.yaml expects: not-found for a missing or
// cross-shop resource (FR-002, SC-007), conflict for a duplicate active
// name, and unprocessable-entity for a validation rule violation.
func mapCatalogError(err error) error {
	var validationErr *service.ValidationError

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return api.NewAPIError(http.StatusNotFound, "not_found", "not found")
	case errors.Is(err, service.ErrDuplicateName):
		return api.NewAPIError(http.StatusConflict, "duplicate_name", err.Error())
	case errors.As(err, &validationErr):
		return api.NewAPIError(http.StatusUnprocessableEntity, "invalid_request", err.Error())
	default:
		return err
	}
}
