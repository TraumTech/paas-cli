package authlogincommandcli

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
	"go.uber.org/mock/gomock"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

// rootWith собирает корневую команду с подкомандой `auth login`, как это делает app.
func rootWith(browser BrowserLogin, out *bytes.Buffer) *cli.Command {
	return &cli.Command{
		Name:   "paas-cli",
		Writer: out,
		Commands: []*cli.Command{{
			Name:     "auth",
			Commands: []*cli.Command{New(browser).CLICommand()},
		}},
	}
}

func TestCommandRun_ReportsWhom(t *testing.T) {
	ctrl := gomock.NewController(t)
	browser := NewMockBrowserLogin(ctrl)
	browser.EXPECT().Execute(gomock.Any()).Return(&usecases.BrowserLoginResult{Email: "user@example.com"}, nil)

	var out bytes.Buffer
	err := rootWith(browser, &out).Run(context.Background(), []string{"paas-cli", "auth", "login"})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Вы вошли как user@example.com")
}

// Вход без e-mail (сервер авторизации его не положил) всё равно отчитывается.
func TestCommandRun_WithoutEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	browser := NewMockBrowserLogin(ctrl)
	browser.EXPECT().Execute(gomock.Any()).Return(&usecases.BrowserLoginResult{}, nil)

	var out bytes.Buffer
	err := rootWith(browser, &out).Run(context.Background(), []string{"paas-cli", "auth", "login"})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "Вы вошли как владелец учётной записи")
}

// Неподтверждённый в браузере вход доезжает до пользователя как есть.
func TestCommandRun_Denied(t *testing.T) {
	ctrl := gomock.NewController(t)
	browser := NewMockBrowserLogin(ctrl)
	browser.EXPECT().Execute(gomock.Any()).Return(nil, entities.ErrAuthorizationDenied)

	err := rootWith(browser, &bytes.Buffer{}).Run(context.Background(), []string{"paas-cli", "auth", "login"})

	assert.ErrorIs(t, err, entities.ErrAuthorizationDenied)
}
