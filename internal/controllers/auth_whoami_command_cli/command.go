package authwhoamicommandcli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

type Command struct {
	whoami CurrentUser
}

func New(whoami CurrentUser) *Command {
	return &Command{whoami: whoami}
}

// CLICommand описывает подкоманду `whoami` для urfave/cli.
func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:   "whoami",
		Usage:  "показать, под кем выполнен вход",
		Action: c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	result, err := c.whoami.Execute(ctx)
	if err != nil {
		return err
	}
	out := cmd.Root().Writer
	if result.Email == "" && result.UserID != "" {
		fmt.Fprintf(out, "✓ Вы вошли как пользователь %s\n", result.UserID)
		return nil
	}
	fmt.Fprintf(out, "✓ Вы вошли как %s\n", result.Email)
	return nil
}
