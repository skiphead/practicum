package utils

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCreateAndParseSessionToken(t *testing.T) {
	cfg := TokenConfig{SessionKey: "test-secret-key"}
	userID := GenerateUserID()

	// Создаём токен
	token, err := CreateSessionToken(userID, cfg)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Парсим токен
	claims, err := ParseSessionToken(token, cfg)
	assert.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.WithinDuration(t, time.Now().Add(SessionDuration), claims.ExpiresAt.Time, time.Second)
}

func TestCreateNewSession_SetsCookie(t *testing.T) {
	cfg := TokenConfig{SessionKey: "test-secret"}
	w := httptest.NewRecorder()

	userID, err := CreateNewSession(w, cfg)
	assert.NoError(t, err)
	assert.NotEmpty(t, userID)

	resp := w.Result()
	defer resp.Body.Close()

	cookies := resp.Cookies()
	assert.Len(t, cookies, 1)
	assert.Equal(t, SessionCookieName, cookies[0].Name)
	assert.True(t, cookies[0].HttpOnly)
}
