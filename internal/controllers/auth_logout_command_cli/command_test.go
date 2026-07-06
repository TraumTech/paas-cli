package authlogoutcommandcli

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
	"go.uber.org/mock/gomock"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

func rootWith(logout UserLogout, out *bytes.Buffer) *cli.Command {
	return &cli.Command{
		Name:   "paas-cli",
		Writer: out,
		Commands: []*cli.Command{{
			Name:     "auth",
			Commands: []*cli.Command{New(logout).CLICommand()},
		}},
	}
}

func TestCommandRun_LoggedOut(t *testing.T) {
	ctrl := gomock.NewController(t)
	logout := NewMockUserLogout(ctrl)
	logout.EXPECT().Execute(gomock.Any()).Return(&usecases.LogoutResult{WasLoggedIn: true}, nil)

	var out bytes.Buffer
	root := rootWith(logout, &out)
	err := root.Run(context.Background(), []string{"paas-cli", "auth", "logout"})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Выход выполнен")
}

func TestCommandRun_WasNotLoggedIn(t *testing.T) {
	ctrl := gomock.NewController(t)
	logout := NewMockUserLogout(ctrl)
	logout.EXPECT().Execute(gomock.Any()).Return(&usecases.LogoutResult{WasLoggedIn: false}, nil)

	var out bytes.Buffer
	root := rootWith(logout, &out)
	err := root.Run(context.Background(), []string{"paas-cli", "auth", "logout"})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "не был выполнен")
}

func TestCommandRun_PropagatesUseCaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	logout := NewMockUserLogout(ctrl)
	execErr := errors.New("network down")
	logout.EXPECT().Execute(gomock.Any()).Return(nil, execErr)

	root := rootWith(logout, &bytes.Buffer{})
	err := root.Run(context.Background(), []string{"paas-cli", "auth", "logout"})

	assert.ErrorIs(t, err, execErr)
}
