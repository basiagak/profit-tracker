// Package handlers holds one file per resource: ingredients, items,
// purchases, sales, reports, auth.
package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/fsetiawan29/profit-tracker/internal/api"
	"github.com/fsetiawan29/profit-tracker/internal/domain"
	"github.com/fsetiawan29/profit-tracker/internal/repository"
)

// maxAuthDateAge rejects Telegram Login Widget payloads older than this, to
// bound replay of a captured payload.
const maxAuthDateAge = 24 * time.Hour

// AuthHandler verifies Telegram Login Widget callbacks and manages the
// resulting session (research.md §5).
type AuthHandler struct {
	botToken string
	users    *repository.UserRepository
	sessions *api.SessionManager
}

// NewAuthHandler builds an AuthHandler.
func NewAuthHandler(botToken string, users *repository.UserRepository, sessions *api.SessionManager) *AuthHandler {
	return &AuthHandler{botToken: botToken, users: users, sessions: sessions}
}

// Register mounts POST /telegram and POST /logout on the open auth group,
// and GET /me on the session-guarded protected group.
func (h *AuthHandler) Register(auth *echo.Group, protected *echo.Group) {
	auth.POST("/telegram", h.telegramLogin)
	auth.POST("/logout", h.logout)
	protected.GET("/me", h.me)
}

type userResponse struct {
	ID          int64   `json:"id"`
	TelegramID  int64   `json:"telegram_id"`
	DisplayName *string `json:"display_name"`
	Username    *string `json:"username"`
}

func newUserResponse(u *domain.User) userResponse {
	return userResponse{
		ID:          u.ID,
		TelegramID:  u.TelegramID,
		DisplayName: u.DisplayName,
		Username:    u.TelegramUsername,
	}
}

func (h *AuthHandler) telegramLogin(c echo.Context) error {
	var raw map[string]interface{}
	dec := json.NewDecoder(c.Request().Body)
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return api.NewAPIError(http.StatusBadRequest, "invalid_request", "malformed request body")
	}

	hashVal, _ := raw["hash"].(string)
	if hashVal == "" {
		return api.NewAPIError(http.StatusUnauthorized, "unauthenticated", "missing hash")
	}
	idNum, ok := raw["id"].(json.Number)
	if !ok {
		return api.NewAPIError(http.StatusBadRequest, "invalid_request", "id is required")
	}
	authDateNum, ok := raw["auth_date"].(json.Number)
	if !ok {
		return api.NewAPIError(http.StatusBadRequest, "invalid_request", "auth_date is required")
	}

	if !verifyTelegramHash(h.botToken, raw, hashVal) {
		return api.NewAPIError(http.StatusUnauthorized, "unauthenticated", "invalid Telegram signature")
	}

	authDate, err := authDateNum.Int64()
	if err != nil {
		return api.NewAPIError(http.StatusBadRequest, "invalid_request", "auth_date must be an integer")
	}
	if time.Since(time.Unix(authDate, 0)) > maxAuthDateAge {
		return api.NewAPIError(http.StatusUnauthorized, "unauthenticated", "Telegram login payload expired")
	}

	telegramID, err := idNum.Int64()
	if err != nil {
		return api.NewAPIError(http.StatusBadRequest, "invalid_request", "id must be an integer")
	}

	var username *string
	if v, ok := raw["username"].(string); ok && v != "" {
		username = &v
	}

	var displayName *string
	firstName, _ := raw["first_name"].(string)
	if lastName, ok := raw["last_name"].(string); ok && lastName != "" {
		name := strings.TrimSpace(firstName + " " + lastName)
		displayName = &name
	} else if firstName != "" {
		displayName = &firstName
	}

	user, err := h.users.FindOrCreateByTelegramID(telegramID, username, displayName)
	if err != nil {
		return err
	}

	if err := h.sessions.IssueCookie(c, user.ID); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]any{"user": newUserResponse(user)})
}

func (h *AuthHandler) logout(c echo.Context) error {
	h.sessions.ClearCookie(c)
	return c.NoContent(http.StatusNoContent)
}

func (h *AuthHandler) me(c echo.Context) error {
	userID, ok := api.UserIDFromContext(c)
	if !ok {
		return api.NewAPIError(http.StatusUnauthorized, "unauthenticated", "authentication required")
	}
	user, err := h.users.FindByID(userID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"user": newUserResponse(user)})
}

// verifyTelegramHash checks hash against the HMAC-SHA256 data-check-string
// computed from every field in raw except "hash", per Telegram's documented
// Login Widget verification algorithm:
// https://core.telegram.org/widgets/login#checking-authorization
func verifyTelegramHash(botToken string, raw map[string]interface{}, hash string) bool {
	keys := make([]string, 0, len(raw))
	for k := range raw {
		if k == "hash" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", k, fieldToString(raw[k])))
	}
	dataCheckString := strings.Join(lines, "\n")

	secretKey := sha256.Sum256([]byte(botToken))
	mac := hmac.New(sha256.New, secretKey[:])
	mac.Write([]byte(dataCheckString))
	expected := mac.Sum(nil)

	expectedHex := fmt.Sprintf("%x", expected)
	return hmac.Equal([]byte(expectedHex), []byte(strings.ToLower(hash)))
}

func fieldToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case json.Number:
		return val.String()
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", val)
	}
}
