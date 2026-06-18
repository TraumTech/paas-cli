package dependencyregistercommandcli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

const manifestFlag = "manifest"

// defaultManifestPath — манифест по умолчанию ищется в корне репозитория.
const defaultManifestPath = "protocols.toml"

type Command struct {
	registrar DependencyRegistrar
}

func New(registrar DependencyRegistrar) *Command {
	return &Command{registrar: registrar}
}

// CLICommand описывает подкоманду `register` для urfave/cli. Имя своего сервиса
// (потребителя) берётся из манифеста (секция [service]); аргументами приходят данные
// конкретной зависимости — версия, продьюсер и снимок его контракта.
func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:      "register",
		Usage:     "зарегистрировать зависимость версии от контракта продьюсера (потребитель — из манифеста, снимок — из локального файла)",
		ArgsUsage: "<version-id> <producer-service-id> <contract-file>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    manifestFlag,
				Aliases: []string{"f"},
				Value:   defaultManifestPath,
				Usage:   "путь к манифесту с секцией [service] (имя сервиса-потребителя)",
			},
		},
		Action: c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 3 {
		return fmt.Errorf("нужно указать <version-id>, <producer-service-id> и путь к файлу контракта (имя сервиса берётся из манифеста)")
	}

	dependency, err := c.registrar.Execute(ctx, usecases.RegisterDependencyInput{
		VersionID:         cmd.Args().Get(0),
		ProducerServiceID: cmd.Args().Get(1),
		ContractPath:      cmd.Args().Get(2),
		ManifestPath:      cmd.String(manifestFlag),
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.Root().Writer, "✓ Зависимость версии от контракта продьюсера %s зарегистрирована.\n",
		dependency.ProducerServiceID)
	return nil
}
