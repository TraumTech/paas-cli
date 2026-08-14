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
	// У входа личным токеном (AUTH-22) важно ещё и то, каким именно токеном
	// вошли и докуда он действует: их несколько и они истекают.
	if result.TokenName != "" {
		fmt.Fprintf(out, "✓ Вы вошли как %s — по личному токену «%s», действует до %s\n",
			whom(result.Email), result.TokenName, result.ExpiresAt.Local().Format("02.01.2006"))
		return nil
	}
	fmt.Fprintf(out, "✓ Вы вошли как %s\n", result.Email)
	return nil
}

// whom — подпись пользователя: e-mail помнит сам вход, платформа его не знает,
// поэтому у входа, положенного руками, его может не быть.
func whom(email string) string {
	if email == "" {
		return "владелец токена"
	}
	return email
}
