package usecases

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/TraumTech/paas-cli/internal/entities"
)

func TestWhoAmIExecute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	sessions := NewMockSessionStore(ctrl)
	inspector := NewMockSessionInspector(ctrl)

	sessions.EXPECT().Load(gomock.Any()).Return("tok-1", nil)
	inspector.EXPECT().Inspect(gomock.Any(), "tok-1").
		Return(&entities.UserSession{Token: "tok-1", Email: "user@example.com"}, nil)

	got, err := NewWhoAmI(sessions, inspector).Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "user@example.com", got.Email)
}

func TestWhoAmIExecute_NoSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	sessions := NewMockSessionStore(ctrl)
	// Inspect не вызывается — без сохранённого входа проверять нечего.
	inspector := NewMockSessionInspector(ctrl)

	sessions.EXPECT().Load(gomock.Any()).Return("", entities.ErrNoSession)

	_, err := NewWhoAmI(sessions, inspector).Execute(context.Background())

	assert.ErrorIs(t, err, entities.ErrNoSession)
}

func TestWhoAmIExecute_SessionExpired(t *testing.T) {
	ctrl := gomock.NewController(t)
	sessions := NewMockSessionStore(ctrl)
	inspector := NewMockSessionInspector(ctrl)

	sessions.EXPECT().Load(gomock.Any()).Return("tok-1", nil)
	inspector.EXPECT().Inspect(gomock.Any(), "tok-1").Return(nil, entities.ErrSessionExpired)

	_, err := NewWhoAmI(sessions, inspector).Execute(context.Background())

	assert.ErrorIs(t, err, entities.ErrSessionExpired)
}
