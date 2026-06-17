package protocolsynccommandcli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

// DestinationFlag — имя глобального флага директории контрактов (тот же, что у fetch);
// для sync он необязателен и переопределяет директорию из манифеста, только если задан явно.
const DestinationFlag = "destination"

const manifestFlag = "manifest"

// defaultManifestPath — манифест по умолчанию ищется в корне репозитория потребителя.
const defaultManifestPath = "protocols.toml"

type Command struct {
	syncer ProtocolSyncer
}

func New(syncer ProtocolSyncer) *Command {
	return &Command{syncer: syncer}
}

// CLICommand описывает подкоманду `sync` для urfave/cli.
func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:  "sync",
		Usage: "получить все контракты, объявленные в манифесте зависимостей",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    manifestFlag,
				Aliases: []string{"f"},
				Value:   defaultManifestPath,
				Usage:   "путь к манифесту зависимостей",
			},
		},
		Action: c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	var override string
	if cmd.IsSet(DestinationFlag) {
		override = cmd.String(DestinationFlag)
	}

	result, err := c.syncer.Execute(ctx, usecases.SyncProtocolsInput{
		ManifestPath:        cmd.String(manifestFlag),
		DestinationOverride: override,
	})
	if err != nil {
		return err
	}

	for _, p := range result.Protocols {
		fmt.Fprintf(cmd.Root().Writer, "✓ Контракт сервиса %s (версия %d) записан в %s\n",
			p.ServiceName, p.VersionNumber, p.Path)
	}
	fmt.Fprintf(cmd.Root().Writer, "Готово: получено контрактов — %d (директория %s)\n",
		len(result.Protocols), result.Destination)
	return nil
}
