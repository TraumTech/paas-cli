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

func TestLoginExecute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	auth := NewMockCredentialAuthenticator(ctrl)
	sessions := NewMockSessionStore(ctrl)

	auth.EXPECT().Authenticate(gomock.Any(), "user@example.com", "secret").
		Return(&entities.UserSession{Token: "tok-1", Email: "user@example.com"}, nil)
	sessions.EXPECT().Save(gomock.Any(), entities.Credential{
		Kind: entities.CredentialSession, Token: "tok-1", Email: "user@example.com",
	}).Return(nil)

	got, err := NewLogin(auth, sessions).Execute(context.Background(),
		LoginInput{Email: "user@example.com", Password: "secret"})

	require.NoError(t, err)
	assert.Equal(t, "user@example.com", got.Email)
}

func TestLoginExecute_EmptyCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	// Authenticate и Save не вызываются — неполные данные не отправляются провайдеру.
	auth := NewMockCredentialAuthenticator(ctrl)
	sessions := NewMockSessionStore(ctrl)

	uc := NewLogin(auth, sessions)
	for _, in := range []LoginInput{{}, {Email: "user@example.com"}, {Password: "secret"}} {
		_, err := uc.Execute(context.Background(), in)
		assert.ErrorIs(t, err, entities.ErrEmptyCredentials)
	}
}

func TestLoginExecute_InvalidCredentials_NoSave(t *testing.T) {
	ctrl := gomock.NewController(t)
	auth := NewMockCredentialAuthenticator(ctrl)
	sessions := NewMockSessionStore(ctrl)

	auth.EXPECT().Authenticate(gomock.Any(), "user@example.com", "wrong").
		Return(nil, entities.ErrInvalidCredentials)
	// Save не вызывается — прежний вход (если был) не затирается.

	_, err := NewLogin(auth, sessions).Execute(context.Background(),
		LoginInput{Email: "user@example.com", Password: "wrong"})

	assert.ErrorIs(t, err, entities.ErrInvalidCredentials)
}

func TestLoginExecute_SaveError(t *testing.T) {
	ctrl := gomock.NewController(t)
	auth := NewMockCredentialAuthenticator(ctrl)
	sessions := NewMockSessionStore(ctrl)

	saveErr := errors.New("disk full")
	auth.EXPECT().Authenticate(gomock.Any(), "user@example.com", "secret").
		Return(&entities.UserSession{Token: "tok-1", Email: "user@example.com"}, nil)
	sessions.EXPECT().Save(gomock.Any(), gomock.Any()).Return(saveErr)

	_, err := NewLogin(auth, sessions).Execute(context.Background(),
		LoginInput{Email: "user@example.com", Password: "secret"})

	assert.ErrorIs(t, err, saveErr)
}
