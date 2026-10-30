package usecases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/TraumTech/paas-cli/internal/entities"
)

func oidcCredential() *entities.Credential {
	return &entities.Credential{
		Kind:         entities.CredentialOIDC,
		Token:        "access-1",
		RefreshToken: "refresh-1",
		Email:        "user@example.com",
	}
}

func TestWhoAmIExecute_NoSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	sessions := NewMockSessionStore(ctrl)
	// Inspect не вызывается — без сохранённого входа проверять нечего.
	login := NewMockLoginInspector(ctrl)

	sessions.EXPECT().Load(gomock.Any()).Return(nil, entities.ErrNoSession)

	_, err := NewWhoAmI(sessions, login).Execute(context.Background())

	assert.ErrorIs(t, err, entities.ErrNoSession)
}

// Вход OIDC подтверждается у сервера авторизации тем же входом, которым CLI
// ходит на платформу: ответ и есть доказательство, что вход жив.
func TestWhoAmIExecute_OIDC(t *testing.T) {
	ctrl := gomock.NewController(t)
	sessions := NewMockSessionStore(ctrl)
	login := NewMockLoginInspector(ctrl)

	sessions.EXPECT().Load(gomock.Any()).Return(oidcCredential(), nil)
	login.EXPECT().Inspect(gomock.Any()).Return(&entities.UserSession{Email: "user@example.com", UserID: "u-1"}, nil)

	got, err := NewWhoAmI(sessions, login).Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "user@example.com", got.Email)
	assert.Equal(t, "u-1", got.UserID)
}

func TestWhoAmIExecute_OIDC_Expired(t *testing.T) {
	ctrl := gomock.NewController(t)
	sessions := NewMockSessionStore(ctrl)
	login := NewMockLoginInspector(ctrl)

	sessions.EXPECT().Load(gomock.Any()).Return(oidcCredential(), nil)
	login.EXPECT().Inspect(gomock.Any()).Return(nil, entities.ErrSessionExpired)

	_, err := NewWhoAmI(sessions, login).Execute(context.Background())

	assert.ErrorIs(t, err, entities.ErrSessionExpired)
}
