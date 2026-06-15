package versionpublishcommandcli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

type Command struct {
	publisher VersionPublisher
}

func New(publisher VersionPublisher) *Command {
	return &Command{publisher: publisher}
}

// CLICommand описывает подкоманду `publish` для urfave/cli.
func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:      "publish",
		Usage:     "зафиксировать версию сервиса по развёрнутой ревизии коммита",
		ArgsUsage: "<service-id> <commit-revision>",
		Action:    c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 2 {
		return fmt.Errorf("нужно указать <service-id> и <commit-revision>")
	}

	version, err := c.publisher.Execute(ctx, usecases.PublishVersionInput{
		ServiceID:      cmd.Args().Get(0),
		CommitRevision: cmd.Args().Get(1),
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
