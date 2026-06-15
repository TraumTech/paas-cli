package protocolpublishcommandcli

import (
	"context"
	"fmt"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

type Command struct {
	publisher ProtocolPublisher
}

func New(publisher ProtocolPublisher) *Command {
	return &Command{publisher: publisher}
}

// CLICommand описывает подкоманду `publish` для urfave/cli.
func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:      "publish",
		Usage:     "опубликовать протокол сервиса под версией (контракт берётся из локального файла)",
		ArgsUsage: "<service-id> <version-id> <contract-file>",
		Action:    c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 3 {
		return fmt.Errorf("нужно указать <service-id>, <version-id> и путь к файлу контракта")
	}

	publication, err := c.publisher.Execute(ctx, usecases.PublishProtocolInput{
		ServiceID:    cmd.Args().Get(0),
		VersionID:    cmd.Args().Get(1),
		ContractPath: cmd.Args().Get(2),
	})
	if err != nil {
		return err
	}

	render(cmd.Root().Writer, publication)
	return nil
}

// render печатает итог публикации. Сводка совместимости с потребителями только
// информирует: ломающее изменение не делает команду неуспешной — гейт ломающих
// изменений живёт в отдельной проверке совместимости до деплоя.
func render(w io.Writer, p *entities.ProtocolPublication) {
	fmt.Fprintf(w, "Протокол опубликован под версией v%d.\n", p.VersionNumber)

	if len(p.Consumers) == 0 {
		fmt.Fprintln(w, "Потребителей нет — публикация никого не затрагивает.")
		return
	}

	fmt.Fprintf(w, "Совместимость с потребителями (%d):\n", len(p.Consumers))
	for _, consumer := range p.Consumers {
		fmt.Fprintf(w, "• %s v%d: %s\n", consumer.ServiceName, consumer.VersionNumber, consumerStatus(consumer))
		for _, change := range consumer.Changes {
			label := "compatible"
			if change.Breaking {
				label = "BREAKING"
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
		return "несравнимо (снимок не разобран)"
	case c.Breaking:
		return "ЛОМАЕТ"
	case len(c.Changes) == 0:
		return "совместимо, без изменений"
	default:
		return "совместимо"
	}
}
