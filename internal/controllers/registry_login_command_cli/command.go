// Package registrylogincommandcli — `paas-cli registry login`: вход CI сервиса
// в реестр образов платформы без секретов в репозитории (TRA-499).
package registrylogincommandcli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type RegistryLogin interface {
	Execute(ctx context.Context) (*entities.RegistryLogin, error)
}

type Command struct {
	login RegistryLogin
}

func New(l RegistryLogin) *Command {
	return &Command{login: l}
}

func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:  "login",
		Usage: "войти docker'ом в реестр образов платформы от имени CI сервиса; печатает адрес репозитория образов сервиса",
		Description: "В GitHub Actions нужен permissions: id-token: write. Вход действует час и даёт " +
			"право публиковать и читать только образы своего сервиса:\n\n" +
			"  IMAGE=$(paas-cli registry login)\n  docker build -t \"$IMAGE:$GITHUB_SHA\" . && docker push \"$IMAGE:$GITHUB_SHA\"",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			login, err := c.login.Execute(ctx)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.Root().Writer, login.Repository)
			return err
		},
	}
}
