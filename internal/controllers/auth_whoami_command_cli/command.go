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
	fmt.Fprintf(cmd.Root().Writer, "✓ Вы вошли как %s\n", result.Email)
	return nil
}
