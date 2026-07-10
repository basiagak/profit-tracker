package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

const (
	sessionCookieName = "session"
	sessionTTL        = 30 * 24 * time.Hour
)

type contextKey string

const userIDContextKey contextKey = "user_id"

// SessionManager issues and verifies signed, HttpOnly, SameSite=Lax session
// cookies keyed by SESSION_SECRET (research.md §5). The cookie value is
// base64url(payload).base64url(HMAC-SHA256(payload)); there is no
// server-side session store.
type SessionManager struct {
	secret []byte
	secure bool
}

// NewSessionManager builds a SessionManager. secure controls the cookie's
// Secure flag — set true whenever the API is served over HTTPS.
func NewSessionManager(secret string, secure bool) *SessionManager {
	return &SessionManager{secret: []byte(secret), secure: secure}
}

type sessionPayload struct {
	UserID int64 `json:"user_id"`
	Exp    int64 `json:"exp"`
}

func (m *SessionManager) sign(payload []byte) []byte {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write(payload)
	return mac.Sum(nil)
}

// IssueCookie sets a signed session cookie authenticating userID.
func (m *SessionManager) IssueCookie(c echo.Context, userID int64) error {
	payload, err := json.Marshal(sessionPayload{
		UserID: userID,
		Exp:    time.Now().Add(sessionTTL).Unix(),
	})
	if err != nil {
		return err
	}

	sig := m.sign(payload)
	value := base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(sig)

	c.SetCookie(&http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// ClearCookie expires the session cookie (logout).
func (m *SessionManager) ClearCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// VerifyRequest reads and validates the session cookie from the request,
// returning the authenticated user ID. Returns an error if the cookie is
// missing, malformed, forged, or expired.
func (m *SessionManager) VerifyRequest(c echo.Context) (int64, error) {
	cookie, err := c.Cookie(sessionCookieName)
	if err != nil {
		return 0, err
	}

	parts := splitSessionValue(cookie.Value)
	if parts == nil {
		return 0, errInvalidSession
	}
	payload, sig := parts[0], parts[1]

	rawPayload, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return 0, errInvalidSession
	}
	rawSig, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return 0, errInvalidSession
	}

	expectedSig := m.sign(rawPayload)
	if subtle.ConstantTimeCompare(expectedSig, rawSig) != 1 {
		return 0, errInvalidSession
	}

	var p sessionPayload
	if err := json.Unmarshal(rawPayload, &p); err != nil {
		return 0, errInvalidSession
	}
	if time.Now().Unix() > p.Exp {
		return 0, errExpiredSession
	}

	return p.UserID, nil
}

// Middleware returns Echo middleware that verifies the session cookie and,
// on success, stores the authenticated user ID in the request context
// (retrievable via UserIDFromContext). On failure it responds 401 with the
// { "error", "code": "unauthenticated" } shape.
func (m *SessionManager) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID, err := m.VerifyRequest(c)
			if err != nil {
				return NewAPIError(http.StatusUnauthorized, "unauthenticated", "authentication required")
			}
			c.Set(string(userIDContextKey), userID)
			return next(c)
		}
	}
}

// UserIDFromContext retrieves the authenticated user ID stored by
// SessionManager.Middleware.
func UserIDFromContext(c echo.Context) (int64, bool) {
	v := c.Get(string(userIDContextKey))
	userID, ok := v.(int64)
	return userID, ok
}

func splitSessionValue(value string) []string {
	for i := len(value) - 1; i >= 0; i-- {
		if value[i] == '.' {
			return []string{value[:i], value[i+1:]}
		}
	}
	return nil
}

var (
	errInvalidSession = NewAPIError(http.StatusUnauthorized, "unauthenticated", "invalid session")
	errExpiredSession = NewAPIError(http.StatusUnauthorized, "unauthenticated", "session expired")
)
