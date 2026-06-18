package versionpublishcommandcli

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
	publisher VersionPublisher
}

func New(publisher VersionPublisher) *Command {
	return &Command{publisher: publisher}
}

// CLICommand описывает подкоманду `publish` для urfave/cli. Имя сервиса берётся из
// манифеста (секция [service]); аргументом приходит только эфемерная ревизия коммита.
func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:      "publish",
		Usage:     "зафиксировать версию сервиса по развёрнутой ревизии коммита (сервис — из манифеста)",
		ArgsUsage: "<commit-revision>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    manifestFlag,
				Aliases: []string{"f"},
				Value:   defaultManifestPath,
				Usage:   "путь к манифесту с секцией [service] (имя сервиса)",
			},
		},
		Action: c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("нужно указать <commit-revision> (имя сервиса берётся из манифеста)")
	}

	version, err := c.publisher.Execute(ctx, usecases.PublishVersionInput{
		CommitRevision: cmd.Args().Get(0),
		ManifestPath:   cmd.String(manifestFlag),
	})
	if err != nil {
		return err
	}

	// Идентификатор версии — на stdout отдельной строкой: его без разбора
	// подхватывает следующий шаг выкатки (`id=$(paas-cli versions publish …)`).
	// Человекочитаемое подтверждение уходит на stderr, чтобы не мешать автоматике.
	fmt.Fprintln(cmd.Root().Writer, version.ID)
	fmt.Fprintf(cmd.Root().ErrWriter, "✓ Версия %d зафиксирована для ревизии %s\n",
		version.Number, version.CommitRevision)
	return nil
}
