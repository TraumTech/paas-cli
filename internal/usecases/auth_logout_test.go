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

func TestLogoutExecute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	sessions := NewMockSessionStore(ctrl)
	revoker := NewMockSessionRevoker(ctrl)

	sessions.EXPECT().Load(gomock.Any()).Return("tok-1", nil)
	revoker.EXPECT().Revoke(gomock.Any(), "tok-1").Return(nil)
	sessions.EXPECT().Delete(gomock.Any()).Return(nil)

	got, err := NewLogout(sessions, revoker).Execute(context.Background())

	require.NoError(t, err)
	assert.True(t, got.WasLoggedIn)
}

func TestLogoutExecute_NoSession_NotAnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	sessions := NewMockSessionStore(ctrl)
	// Revoke и Delete не вызываются — выходить не из чего.
	revoker := NewMockSessionRevoker(ctrl)

	sessions.EXPECT().Load(gomock.Any()).Return("", entities.ErrNoSession)

	got, err := NewLogout(sessions, revoker).Execute(context.Background())

	require.NoError(t, err)
	assert.False(t, got.WasLoggedIn)
}

func TestLogoutExecute_RevokeError_KeepsLocalSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	sessions := NewMockSessionStore(ctrl)
	revoker := NewMockSessionRevoker(ctrl)

	revokeErr := errors.New("network down")
	sessions.EXPECT().Load(gomock.Any()).Return("tok-1", nil)
	revoker.EXPECT().Revoke(gomock.Any(), "tok-1").Return(revokeErr)
	// Delete не вызывается — сессия у провайдера жива, выход можно повторить.

	_, err := NewLogout(sessions, revoker).Execute(context.Background())

	assert.ErrorIs(t, err, revokeErr)
}
