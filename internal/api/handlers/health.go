package handlers

import (
	"database/sql"
	"net/http"

	"github.com/labstack/echo/v4"
)

// HealthHandler reports whether the API and its database connection are up.
type HealthHandler struct {
	db *sql.DB
}

// NewHealthHandler builds a HealthHandler.
func NewHealthHandler(db *sql.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

// Register mounts GET /health on e, unauthenticated.
func (h *HealthHandler) Register(e *echo.Echo) {
	e.GET("/health", h.check)
}

func (h *HealthHandler) check(c echo.Context) error {
	if err := h.db.PingContext(c.Request().Context()); err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"status": "error",
			"error":  err.Error(),
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
