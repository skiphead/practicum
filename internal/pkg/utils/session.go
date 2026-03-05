package utils

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Context keys should be unexported to prevent collisions
type contextKey string

const (
	SessionCookieName            = "session_token"
	SessionDuration              = 24 * time.Hour
	KeyUserID         contextKey = "user_id"
)

// SessionClaims represents JWT claims for session management.
type SessionClaims struct {
	jwt.RegisteredClaims
	UserID string `json:"user_id"`
}

// TokenConfig holds configuration for token operations.
type TokenConfig struct {
	SessionKey string
}

// CreateSessionToken generates a new JWT token for the given userID.
// Returns the signed token string or error if generation failed.
func CreateSessionToken(userID string, cfg TokenConfig) (string, error) {
	claims := SessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(SessionDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID: userID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.SessionKey))
}

// ParseSessionToken validates and parses a JWT token string.
// Returns SessionClaims if valid, or error if parsing/validation failed.
func ParseSessionToken(tokenString string, cfg TokenConfig) (*SessionClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &SessionClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(cfg.SessionKey), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(*SessionClaims)
	if !ok || claims.UserID == "" {
		return nil, fmt.Errorf("invalid claims")
	}
	return claims, nil
}

// SetSessionCookie sets an HTTP cookie with the session token.
func SetSessionCookie(w http.ResponseWriter, tokenString string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    tokenString,
		Path:     "/",
		Expires:  time.Now().Add(SessionDuration),
		HttpOnly: true,
		Secure:   false, // Установите true в production с HTTPS
		SameSite: http.SameSiteLaxMode,
	})
}

// GetSessionCookie retrieves the session token from request cookies.
// Returns the token string or error if cookie not found.
func GetSessionCookie(r *http.Request) (string, error) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

// GenerateUserID creates a new unique user identifier.
func GenerateUserID() string {
	return uuid.New().String()
}

// ContextWithUserID returns a new context with the userID stored.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, KeyUserID, userID)
}

// UserIDFromContext extracts the UserID from the request context.
// Returns empty string if not found or invalid type.
func UserIDFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value(KeyUserID).(string); ok {
		return userID
	}
	return ""
}

// CreateNewSession creates a complete new session: generates userID, token, and sets cookie.
// Returns the new userID and error if token creation failed.
func CreateNewSession(w http.ResponseWriter, cfg TokenConfig) (string, error) {
	userID := GenerateUserID()

	tokenString, err := CreateSessionToken(userID, cfg)
	if err != nil {
		zap.L().Error("Failed to create JWT token", zap.Error(err))
		return userID, err
	}

	SetSessionCookie(w, tokenString)
	return userID, nil
}

// GenerateSessionToken создаёт новый JWT токен для указанного userID.
// Это алиас для CreateSessionToken для удобства использования в тестах.
// Возвращает подписанный токен или ошибку при генерации.
func GenerateSessionToken(userID string, cfg TokenConfig) (string, error) {
	return CreateSessionToken(userID, cfg)
}
