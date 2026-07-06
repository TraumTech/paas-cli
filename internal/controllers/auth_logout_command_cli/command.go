package authlogoutcommandcli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

type Command struct {
	logout UserLogout
}

func New(logout UserLogout) *Command {
	return &Command{logout: logout}
}

// CLICommand описывает подкоманду `logout` для urfave/cli.
func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:   "logout",
		Usage:  "выйти: завершить сессию и удалить сохранённый вход",
		Action: c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	result, err := c.logout.Execute(ctx)
	if err != nil {
		return err
	}
	if !result.WasLoggedIn {
		fmt.Fprintln(cmd.Root().Writer, "✓ Вход и не был выполнен — выходить не из чего")
		return nil
	}
	fmt.Fprintln(cmd.Root().Writer, "✓ Выход выполнен: сессия завершена, сохранённый вход удалён")
	return nil
}
