package protocolcompatibilitycommandcli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/urfave/cli/v3"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

// errBreaking — кандидат ломает потребителей. Возвращается из команды, чтобы CLI
// завершился ненулевым кодом и процесс выкатки (CI/CD) остановился до деплоя.
var errBreaking = errors.New("кандидат ломает потребителей — выкатка остановлена")

type Command struct {
	checker CompatibilityChecker
}

func New(checker CompatibilityChecker) *Command {
	return &Command{checker: checker}
}

const formatFlag = "format"

// CLICommand описывает подкоманду `compatibility` для urfave/cli.
func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:      "compatibility",
		Aliases:   []string{"compat"},
		Usage:     "проверить совместимость контракта-кандидата с потребителями (без публикации)",
		ArgsUsage: "<service-id> <candidate-file>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  formatFlag,
				Usage: "формат кандидата: openapi (по умолчанию) или grpc (.proto-исходник)",
			},
		},
		Action: c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 2 {
		return fmt.Errorf("нужно указать <service-id> и путь к файлу контракта-кандидата")
	}
	// Формат проверяем до чтения файла и похода на платформу: опечатка — ошибка
	// сразу, а не молчаливая проверка не тем типом (как в publish, CLI-18).
	format, err := entities.ParseProtocolFormat(cmd.String(formatFlag))
	if err != nil {
		return err
	}

	report, err := c.checker.Execute(ctx, usecases.CheckCompatibilityInput{
		ServiceID:     cmd.Args().Get(0),
		Format:        format,
		CandidatePath: cmd.Args().Get(1),
	})
	if err != nil {
		return err
	}

	return render(cmd.Root().Writer, report)
}

func render(w io.Writer, report *entities.CompatibilityReport) error {
	if len(report.Consumers) == 0 {
		fmt.Fprintln(w, "Потребителей нет — кандидат никого не затрагивает.")
		return nil
	}

	fmt.Fprintf(w, "Совместимость кандидата с потребителями (%d):\n", len(report.Consumers))
	for _, consumer := range report.Consumers {
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

	if report.Breaking {
		return errBreaking
	}
	return nil
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
