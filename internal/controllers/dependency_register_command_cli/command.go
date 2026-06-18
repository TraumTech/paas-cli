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

// CLICommand описывает подкоманду `register` для urfave/cli. Весь состав зависимостей
// и имя своего сервиса (потребителя) берутся из манифеста; продьюсеры резолвятся по
// имени, снимки — из раскладки контрактов. Аргументом приходит только версия.
func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:      "register",
		Usage:     "зарегистрировать зависимости версии от контрактов продьюсеров, объявленных в манифесте",
		ArgsUsage: "<version-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    manifestFlag,
				Aliases: []string{"f"},
				Value:   defaultManifestPath,
				Usage:   "путь к манифесту зависимостей с секцией [service]",
			},
		},
		Action: c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("нужно указать <version-id> (состав зависимостей берётся из манифеста)")
	}

	result, err := c.registrar.Execute(ctx, usecases.RegisterDependencyInput{
		VersionID:    cmd.Args().Get(0),
		ManifestPath: cmd.String(manifestFlag),
	})
	if err != nil {
		return err
	}

	for _, dep := range result.Registered {
		fmt.Fprintf(cmd.Root().Writer, "✓ Зависимость от %s (%s) зарегистрирована.\n",
			dep.ProducerName, dep.ProducerServiceID)
	}
	fmt.Fprintf(cmd.Root().Writer, "Готово: зарегистрировано зависимостей — %d.\n", len(result.Registered))
	return nil
}
