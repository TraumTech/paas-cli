package authlogincommandcli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

type Command struct {
	browser BrowserLogin
}

func New(browser BrowserLogin) *Command {
	return &Command{browser: browser}
}

// CLICommand описывает подкоманду `login` для urfave/cli.
func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:   "login",
		Usage:  "войти под своей учётной записью платформы через браузер",
		Action: c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	result, err := c.browser.Execute(ctx)
	if err != nil {
		return err
	}
	whom := result.Email
	if whom == "" {
		whom = "владелец учётной записи"
	}
	fmt.Fprintf(cmd.Root().Writer, "✓ Вы вошли как %s — вход сохранён, команды выполняются от вашего имени\n", whom)
	return nil
}
