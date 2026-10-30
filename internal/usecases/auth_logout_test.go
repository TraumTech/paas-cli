package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// Вход OIDC гасится у сервера авторизации по refresh-токену — вместе с ним
// умирают и токены доступа; сессию браузера выход CLI не трогает.
func TestLogoutExecute_OIDC_RevokesRefreshToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	sessions := NewMockSessionStore(ctrl)
	login := NewMockLoginRevoker(ctrl)

	sessions.EXPECT().Load(gomock.Any()).Return(oidcCredential(), nil)
	login.EXPECT().Revoke(gomock.Any(), "refresh-1").Return(nil)
	sessions.EXPECT().Delete(gomock.Any()).Return(nil)

	got, err := NewLogout(sessions, login).Execute(context.Background())

	require.NoError(t, err)
	assert.True(t, got.WasLoggedIn)
}

// Нет входа (или он прежнего вида) — отзывать нечего, файл убирается.
func TestLogoutExecute_NoSession_NotAnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	sessions := NewMockSessionStore(ctrl)
	login := NewMockLoginRevoker(ctrl)

	sessions.EXPECT().Load(gomock.Any()).Return(nil, entities.ErrNoSession)
	sessions.EXPECT().Delete(gomock.Any()).Return(nil)

	got, err := NewLogout(sessions, login).Execute(context.Background())

	require.NoError(t, err)
	assert.False(t, got.WasLoggedIn)
}

func TestLogoutExecute_RevokeError_KeepsLocalSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	sessions := NewMockSessionStore(ctrl)
	login := NewMockLoginRevoker(ctrl)

	// Delete не вызывается: отзыв не прошёл — вход остаётся, выход можно повторить.
	sessions.EXPECT().Load(gomock.Any()).Return(oidcCredential(), nil)
	login.EXPECT().Revoke(gomock.Any(), "refresh-1").Return(errors.New("network down"))

	_, err := NewLogout(sessions, login).Execute(context.Background())

	assert.Error(t, err)
}
