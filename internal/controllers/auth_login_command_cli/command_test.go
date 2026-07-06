package authlogincommandcli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
	"go.uber.org/mock/gomock"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

// rootWith собирает корневую команду с подкомандой `auth login`, как это делает
// app; stdin подменяется ридером — пароль (и e-mail без флага) читаются из него.
func rootWith(login UserLogin, in string, out *bytes.Buffer) *cli.Command {
	return &cli.Command{
		Name:   "paas-cli",
		Writer: out,
		Reader: strings.NewReader(in),
		Commands: []*cli.Command{{
			Name:     "auth",
			Commands: []*cli.Command{New(login).CLICommand()},
		}},
	}
}

func TestCommandRun_EmailFlag_PromptsOnlyPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	login := NewMockUserLogin(ctrl)
	login.EXPECT().
		Execute(gomock.Any(), usecases.LoginInput{Email: "user@example.com", Password: "secret"}).
		Return(&usecases.LoginResult{Email: "user@example.com"}, nil)

	var out bytes.Buffer
	root := rootWith(login, "secret\n", &out)
	err := root.Run(context.Background(), []string{"paas-cli", "auth", "login", "--email", "user@example.com"})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Вы вошли как user@example.com")
	assert.NotContains(t, out.String(), "E-mail:")
}

func TestCommandRun_PromptsEmailAndPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	login := NewMockUserLogin(ctrl)
	login.EXPECT().
		Execute(gomock.Any(), usecases.LoginInput{Email: "user@example.com", Password: "secret"}).
		Return(&usecases.LoginResult{Email: "user@example.com"}, nil)

	var out bytes.Buffer
	root := rootWith(login, "user@example.com\nsecret\n", &out)
	err := root.Run(context.Background(), []string{"paas-cli", "auth", "login"})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "E-mail:")
	assert.Contains(t, out.String(), "Пароль:")
}

func TestCommandRun_TrimsEmailWhitespace(t *testing.T) {
	ctrl := gomock.NewController(t)
	login := NewMockUserLogin(ctrl)
	login.EXPECT().
		Execute(gomock.Any(), usecases.LoginInput{Email: "user@example.com", Password: "secret"}).
		Return(&usecases.LoginResult{Email: "user@example.com"}, nil)

	root := rootWith(login, "  user@example.com  \nsecret\n", &bytes.Buffer{})
	require.NoError(t, root.Run(context.Background(), []string{"paas-cli", "auth", "login"}))
}

func TestCommandRun_PropagatesUseCaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	login := NewMockUserLogin(ctrl)
	login.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, entities.ErrInvalidCredentials)

	root := rootWith(login, "wrong\n", &bytes.Buffer{})
	err := root.Run(context.Background(), []string{"paas-cli", "auth", "login", "-e", "user@example.com"})

	assert.ErrorIs(t, err, entities.ErrInvalidCredentials)
}
