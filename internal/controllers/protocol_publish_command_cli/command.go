package protocolpublishcommandcli

import (
	"context"
	"fmt"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

const manifestFlag = "manifest"

// defaultManifestPath — манифест по умолчанию ищется в корне репозитория владельца.
// Пустой путь — выбор манифеста за читателем: paas.toml, при его
// отсутствии — переходный protocols.toml (CLI-22).
const defaultManifestPath = ""

type Command struct {
	publisher ProtocolPublisher
}

func New(publisher ProtocolPublisher) *Command {
	return &Command{publisher: publisher}
}

// CLICommand описывает подкоманду `publish` для urfave/cli. Имя сервиса и путь к его
// контракту берутся из манифеста (секция [service]); аргументом приходит только
// эфемерная версия, под которой публикуется протокол.
func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:      "publish",
		Usage:     "опубликовать протокол сервиса под версией (сервис и контракт — из манифеста)",
		ArgsUsage: "<version-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    manifestFlag,
				Aliases: []string{"f"},
				Value:   defaultManifestPath,
				Usage:   "путь к манифесту (по умолчанию paas.toml, при его отсутствии protocols.toml)",
			},
		},
		Action: c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("нужно указать <version-id> (имя сервиса и контракт берутся из манифеста)")
	}

	report, err := c.publisher.Execute(ctx, usecases.PublishProtocolInput{
		VersionID:    cmd.Args().Get(0),
		ManifestPath: cmd.String(manifestFlag),
	})
	if err != nil {
		return err
	}

	for i := range report.Publications {
		render(cmd.Root().Writer, &report.Publications[i])
	}
	for _, name := range report.Orphaned {
		fmt.Fprintf(cmd.Root().Writer, "Внимание: протокол %q остался в реестре, но исчез из манифеста — эта публикация его не обновляла.\n", name)
	}
	return nil
}

// render печатает итог одной публикации. Сводка совместимости с потребителями
// только информирует: ломающее изменение не делает команду неуспешной — гейт
// ломающих изменений живёт в отдельной проверке совместимости до деплоя.
func render(w io.Writer, p *entities.ProtocolPublication) {
	if p.Name != "" {
		fmt.Fprintf(w, "Протокол %q опубликован под версией v%d.\n", p.Name, p.VersionNumber)
	} else {
		fmt.Fprintf(w, "Протокол опубликован под версией v%d.\n", p.VersionNumber)
	}

	if len(p.Consumers) == 0 {
		fmt.Fprintln(w, "Потребителей нет — публикация никого не затрагивает.")
		return
	}

	fmt.Fprintf(w, "Совместимость с потребителями (%d):\n", len(p.Consumers))
	for _, consumer := range p.Consumers {
		fmt.Fprintf(w, "• %s v%d: %s\n", consumer.ServiceName, consumer.VersionNumber, consumerStatus(consumer))
		for _, change := range consumer.Changes {
			label := "compatible"
			switch {
			case change.Breaking:
				label = "BREAKING"
			case change.Waived:
				// Изменение осталось видно, но потребитель от этого атрибута
				// отказался (PRT-27) — его оно уже не ломает.
				label = "отказ"
			}
			operation := change.Operation
			if operation != "" {
				operation = " " + operation
			}
			fmt.Fprintf(w, "    [%s] %s%s — %s\n", label, change.Kind, operation, change.Description)
		}
	}

	if p.Breaking {
		fmt.Fprintln(w, "Внимание: новый контракт ломает часть потребителей — см. список выше.")
	}
}

func consumerStatus(c entities.ConsumerCompatibility) string {
	switch {
	case !c.Comparable:
		return "не сверялось (снимок несравним с контрактом)"
	case c.Breaking:
		return "ЛОМАЕТ"
	case len(c.Changes) == 0:
		return "совместимо, без изменений"
	default:
		return "совместимо"
	}
}
