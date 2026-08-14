package authwhoamicommandcli

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
	"go.uber.org/mock/gomock"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

func rootWith(whoami CurrentUser, out *bytes.Buffer) *cli.Command {
	return &cli.Command{
		Name:   "paas-cli",
		Writer: out,
		Commands: []*cli.Command{{
			Name:     "auth",
			Commands: []*cli.Command{New(whoami).CLICommand()},
		}},
	}
}

func TestCommandRun_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	whoami := NewMockCurrentUser(ctrl)
	whoami.EXPECT().Execute(gomock.Any()).Return(&usecases.WhoAmIResult{Email: "user@example.com"}, nil)

	var out bytes.Buffer
	root := rootWith(whoami, &out)
	err := root.Run(context.Background(), []string{"paas-cli", "auth", "whoami"})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Вы вошли как user@example.com")
}

func TestCommandRun_PropagatesUseCaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	whoami := NewMockCurrentUser(ctrl)
	whoami.EXPECT().Execute(gomock.Any()).Return(nil, entities.ErrNoSession)

	root := rootWith(whoami, &bytes.Buffer{})
	err := root.Run(context.Background(), []string{"paas-cli", "auth", "whoami"})

	assert.ErrorIs(t, err, entities.ErrNoSession)
}

// Вход личным токеном (AUTH-22): кроме владельца показываем, каким токеном
// вошли и докуда он действует — токенов бывает несколько и они истекают.
func TestCommandRun_PersonalToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	whoami := NewMockCurrentUser(ctrl)
	whoami.EXPECT().Execute(gomock.Any()).Return(&usecases.WhoAmIResult{
		Email:     "user@example.com",
		TokenName: "ноутбук",
		ExpiresAt: time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC),
	}, nil)

	var out bytes.Buffer
	root := rootWith(whoami, &out)
	require.NoError(t, root.Run(context.Background(), []string{"paas-cli", "auth", "whoami"}))

	assert.Contains(t, out.String(), "user@example.com")
	assert.Contains(t, out.String(), "«ноутбук»")
}
