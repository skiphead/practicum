package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/skiphead/practicum/internal/pkg/utils"
	"go.uber.org/zap"
)

const (
	sessionCookieName = "session_token"
	sessionDuration   = 24 * time.Hour
)

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

// sessionMiddleware — HTTP middleware для управления сессиями.
func (h *URLHandler) sessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := h.getOrCreateSession(w, r)
		if err != nil {
			http.Error(w, "Session error", http.StatusInternalServerError)
			return
		}

		ctx := utils.ContextWithUserID(r.Context(), userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getOrCreateSession извлекает существующую сессию или создаёт новую.
func (h *URLHandler) getOrCreateSession(w http.ResponseWriter, r *http.Request) (string, error) {
	cfg := TokenConfig{SessionKey: h.sessionKey}

	// Попытка получить токен из cookie
	tokenString, err := utils.GetSessionCookie(r)
	if err != nil {
		// Cookie нет — создаём новую сессию
		return utils.CreateNewSession(w, utils.TokenConfig(cfg))
	}

	// Парсим и валидируем токен
	claims, err := utils.ParseSessionToken(tokenString, utils.TokenConfig(cfg))
	if err != nil {
		zap.L().Debug("Invalid token, creating new session", zap.Error(err))
		return utils.CreateNewSession(w, utils.TokenConfig(cfg))
	}

	return claims.UserID, nil
}

// getUserIDFromContext — хелпер для получения UserID из контекста.
func (h *URLHandler) getUserIDFromContext(ctx context.Context) string {
	return utils.UserIDFromContext(ctx)
}
