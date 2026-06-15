package dependencyregistercommandcli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

type Command struct {
	registrar DependencyRegistrar
}

func New(registrar DependencyRegistrar) *Command {
	return &Command{registrar: registrar}
}

// CLICommand описывает подкоманду `register` для urfave/cli.
func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:      "register",
		Usage:     "зарегистрировать зависимость версии от контракта продьюсера (снимок берётся из локального файла)",
		ArgsUsage: "<service-id> <version-id> <producer-service-id> <contract-file>",
		Action:    c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 4 {
		return fmt.Errorf("нужно указать <service-id>, <version-id>, <producer-service-id> и путь к файлу контракта")
	}

	dependency, err := c.registrar.Execute(ctx, usecases.RegisterDependencyInput{
		ServiceID:         cmd.Args().Get(0),
		VersionID:         cmd.Args().Get(1),
		ProducerServiceID: cmd.Args().Get(2),
		ContractPath:      cmd.Args().Get(3),
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.Root().Writer, "✓ Зависимость версии от контракта продьюсера %s зарегистрирована.\n",
		dependency.ProducerServiceID)
	return nil
}
