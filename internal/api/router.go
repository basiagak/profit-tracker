// Package api wires the JSON-only HTTP surface: Echo bootstrap, session
// cookies, and route registration. No html/template or server-rendered view
// exists in this package (research.md §1) — every handler responds via
// c.JSON.
package api

import (
	"errors"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// ErrorResponse is the JSON error shape for every /api/* route:
// { "error": "...", "code": "..." } (contracts/web-api.yaml Error schema).
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// APIError pairs an HTTP status with the machine-readable code slug the
// contract requires (e.g. invalid_quantity, duplicate_name, not_found).
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string { return e.Message }

// NewAPIError builds an APIError to be returned from a handler.
func NewAPIError(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}

// NewEcho builds the Echo instance with recovery/logger/CORS middleware and
// the JSON error handler shape required by contracts/web-api.yaml.
// allowOrigins configures Access-Control-Allow-Origin for the dashboard's
// out-of-scope client; pass explicit origins (not "*") since the API relies
// on credentialed (cookie) requests.
func NewEcho(allowOrigins []string) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(middleware.Recover())
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:   true,
		LogURI:      true,
		LogMethod:   true,
		LogLatency:  true,
		LogError:    true,
		HandleError: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			if v.Error != nil {
				log.Printf("%s %s status=%d latency=%s error=%v", v.Method, v.URI, v.Status, v.Latency, v.Error)
			} else {
				log.Printf("%s %s status=%d latency=%s", v.Method, v.URI, v.Status, v.Latency)
			}
			return nil
		},
	}))
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     allowOrigins,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete},
		AllowCredentials: true,
	}))

	e.HTTPErrorHandler = jsonErrorHandler
	return e
}

func jsonErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	status := http.StatusInternalServerError
	code := "internal_error"
	message := "internal server error"

	var apiErr *APIError
	var httpErr *echo.HTTPError
	switch {
	case errors.As(err, &apiErr):
		status = apiErr.Status
		code = apiErr.Code
		message = apiErr.Message
	case errors.As(err, &httpErr):
		status = httpErr.Code
		code = codeForStatus(status)
		if msg, ok := httpErr.Message.(string); ok {
			message = msg
		}
	}

	if jsonErr := c.JSON(status, ErrorResponse{Error: message, Code: code}); jsonErr != nil {
		c.Logger().Error(jsonErr)
	}
}

func codeForStatus(status int) string {
	switch status {
	case http.StatusNotFound:
		return "not_found"
	case http.StatusUnauthorized:
		return "unauthenticated"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return "invalid_request"
	case http.StatusConflict:
		return "conflict"
	default:
		return "internal_error"
	}
}

// RouteGroups exposes the two route roots handlers register into: Auth
// (open, /api/auth/*) and Protected (session-guarded, /api/*).
type RouteGroups struct {
	Auth      *echo.Group
	Protected *echo.Group
}

// NewRouteGroups mounts /api/auth (open) and /api (guarded by
// sessionMiddleware) on e.
func NewRouteGroups(e *echo.Echo, sessionMiddleware echo.MiddlewareFunc) *RouteGroups {
	root := e.Group("/api")
	return &RouteGroups{
		Auth:      root.Group("/auth"),
		Protected: root.Group("", sessionMiddleware),
	}
}
