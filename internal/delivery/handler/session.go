package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	sessionCookieName = "session_token"
	sessionDuration   = 24 * time.Hour
)

type contextKey string

const (
	keyUserID contextKey = "user_id"
)

type SessionClaims struct {
	jwt.RegisteredClaims
	UserID string `json:"user_id"`
}

func (h *URLHandler) sessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var userID string

		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			userID = h.createNewSession(w)
			ctx := context.WithValue(r.Context(), keyUserID, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		token, err := jwt.ParseWithClaims(cookie.Value, &SessionClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(h.sessionKey), nil
		})

		if err != nil || !token.Valid {
			userID = h.createNewSession(w)
			ctx := context.WithValue(r.Context(), keyUserID, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		if claims, ok := token.Claims.(*SessionClaims); ok {
			if claims.UserID == "" {
				http.Error(w, "Invalid session token", http.StatusUnauthorized)
				return
			}
			userID = claims.UserID
		} else {
			userID = h.createNewSession(w)
		}

		ctx := context.WithValue(r.Context(), keyUserID, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *URLHandler) createNewSession(w http.ResponseWriter) string {
	userID := uuid.New().String()

	claims := SessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(sessionDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID: userID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(h.sessionKey))
	if err != nil {
		zap.L().Error("Failed to create JWT token", zap.Error(err))
		return userID
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    tokenString,
		Path:     "/",
		Expires:  time.Now().Add(sessionDuration),
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})

	return userID
}

func (h *URLHandler) getUserIDFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value(keyUserID).(string); ok {
		return userID
	}
	return ""
}
