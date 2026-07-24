package protocolsynccommandcli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/TraumTech/paas-cli/internal/entities"
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
		note := ""
		if p.NarrowingSkipped {
			note = " — целиком: сужение по методам для этого формата не поддерживается, methods учитываются при регистрации зависимостей"
		}
		fmt.Fprintf(cmd.Root().Writer, "✓ Контракт сервиса %s (версия %d%s) записан в %s%s\n",
			p.ServiceName, p.VersionNumber, formatLabel(p.Format), p.Path, note)
	}
	fmt.Fprintf(cmd.Root().Writer, "Готово: получено контрактов — %d (директория %s)\n",
		len(result.Protocols), result.Destination)
	return nil
}

// formatLabel — пометка формата в отчёте; прежний формат (OpenAPI) не помечаем,
// чтобы привычный вывод не менялся.
func formatLabel(f entities.ProtocolFormat) string {
	if f == entities.ProtocolFormatGRPC {
		return ", gRPC"
	}
	return ""
}
