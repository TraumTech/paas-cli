package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/TraumTech/paas-cli/internal/entities"
)

func sessionCredential(token string) *entities.Credential {
	return &entities.Credential{Kind: entities.CredentialSession, Token: token}
}

func oidcCredential() *entities.Credential {
	return &entities.Credential{
		Kind:         entities.CredentialOIDC,
		Token:        "access-1",
		RefreshToken: "refresh-1",
		Email:        "user@example.com",
	}
}

func personalCredential(tokenID string) *entities.Credential {
	return &entities.Credential{
		Kind:    entities.CredentialPersonalToken,
		Token:   "paas_uat_secret",
		TokenID: tokenID,
		Email:   "user@example.com",
	}
}

func TestWhoAmIExecute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	sessions := NewMockSessionStore(ctrl)
	inspector := NewMockSessionInspector(ctrl)

	tokens := NewMockPersonalTokenDirectory(ctrl)

	sessions.EXPECT().Load(gomock.Any()).Return(sessionCredential("tok-1"), nil)
	inspector.EXPECT().Inspect(gomock.Any(), "tok-1").
		Return(&entities.UserSession{Token: "tok-1", Email: "user@example.com"}, nil)

	got, err := NewWhoAmI(sessions, NewMockLoginInspector(ctrl), inspector, tokens).Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "user@example.com", got.Email)
}

func TestWhoAmIExecute_NoSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	sessions := NewMockSessionStore(ctrl)
	// Inspect не вызывается — без сохранённого входа проверять нечего.
	inspector := NewMockSessionInspector(ctrl)

	tokens := NewMockPersonalTokenDirectory(ctrl)

	sessions.EXPECT().Load(gomock.Any()).Return(nil, entities.ErrNoSession)

	_, err := NewWhoAmI(sessions, NewMockLoginInspector(ctrl), inspector, tokens).Execute(context.Background())

	assert.ErrorIs(t, err, entities.ErrNoSession)
}

func TestWhoAmIExecute_SessionExpired(t *testing.T) {
	ctrl := gomock.NewController(t)
	sessions := NewMockSessionStore(ctrl)
	inspector := NewMockSessionInspector(ctrl)

	tokens := NewMockPersonalTokenDirectory(ctrl)

	sessions.EXPECT().Load(gomock.Any()).Return(sessionCredential("tok-1"), nil)
	inspector.EXPECT().Inspect(gomock.Any(), "tok-1").Return(nil, entities.ErrSessionExpired)

	_, err := NewWhoAmI(sessions, NewMockLoginInspector(ctrl), inspector, tokens).Execute(context.Background())

	assert.ErrorIs(t, err, entities.ErrSessionExpired)
}

// Вход личным токеном подтверждается самой платформой: перечень токенов виден
// только владельцу, а имя и срок берутся из него — так видно, каким именно
// токеном вошли.
func TestWhoAmIExecute_PersonalToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	sessions := NewMockSessionStore(ctrl)
	inspector := NewMockSessionInspector(ctrl) // не вызывается: провайдер про личный токен не знает
	tokens := NewMockPersonalTokenDirectory(ctrl)

	expiresAt := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	sessions.EXPECT().Load(gomock.Any()).Return(personalCredential("tok-id"), nil)
	tokens.EXPECT().List(gomock.Any()).Return([]entities.PersonalToken{
		{ID: "other", Name: "сервер"},
		{ID: "tok-id", Name: "ноутбук", ExpiresAt: expiresAt},
	}, nil)

	got, err := NewWhoAmI(sessions, NewMockLoginInspector(ctrl), inspector, tokens).Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "user@example.com", got.Email)
	assert.Equal(t, "ноутбук", got.TokenName)
	assert.Equal(t, expiresAt, got.ExpiresAt)
}

// Токен отозвали из интерфейса — на платформе его больше нет, и вход об этом
// честно сообщает, а не молчит.
func TestWhoAmIExecute_PersonalTokenRevoked(t *testing.T) {
	ctrl := gomock.NewController(t)
	sessions := NewMockSessionStore(ctrl)
	inspector := NewMockSessionInspector(ctrl)
	tokens := NewMockPersonalTokenDirectory(ctrl)

	sessions.EXPECT().Load(gomock.Any()).Return(personalCredential("tok-id"), nil)
	tokens.EXPECT().List(gomock.Any()).Return([]entities.PersonalToken{{ID: "other"}}, nil)

	_, err := NewWhoAmI(sessions, NewMockLoginInspector(ctrl), inspector, tokens).Execute(context.Background())

	assert.ErrorIs(t, err, entities.ErrPersonalTokenRejected)
}

// Вход OIDC подтверждается у сервера авторизации тем же входом, которым CLI
// ходит на платформу: ответ и есть доказательство, что вход жив.
func TestWhoAmIExecute_OIDC(t *testing.T) {
	ctrl := gomock.NewController(t)
	sessions := NewMockSessionStore(ctrl)
	login := NewMockLoginInspector(ctrl)

	sessions.EXPECT().Load(gomock.Any()).Return(oidcCredential(), nil)
	login.EXPECT().Inspect(gomock.Any()).Return(&entities.UserSession{Email: "user@example.com", UserID: "u-1"}, nil)

	got, err := NewWhoAmI(sessions, login, NewMockSessionInspector(ctrl), NewMockPersonalTokenDirectory(ctrl)).
		Execute(context.Background())

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

	_, err := NewWhoAmI(sessions, login, NewMockSessionInspector(ctrl), NewMockPersonalTokenDirectory(ctrl)).
		Execute(context.Background())

	assert.ErrorIs(t, err, entities.ErrSessionExpired)
}
