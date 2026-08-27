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
	checker  CompatibilityChecker
	manifest ManifestCompatibilityChecker
}

func New(checker CompatibilityChecker, manifest ManifestCompatibilityChecker) *Command {
	return &Command{checker: checker, manifest: manifest}
}

const (
	formatFlag   = "format"
	nameFlag     = "name"
	manifestFlag = "manifest"
)

// CLICommand описывает подкоманду `compatibility` для urfave/cli. Без аргументов
// проверяются все контракты манифеста против их протоколов (CLI-23); с
// аргументами — прежний точечный режим: явный сервис и файл кандидата.
func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:      "compatibility",
		Aliases:   []string{"compat"},
		Usage:     "проверить совместимость контрактов-кандидатов с потребителями (без публикации)",
		ArgsUsage: "[<service-id> <candidate-file>]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  formatFlag,
				Usage: "формат кандидата: openapi (по умолчанию) или grpc (.proto-исходник); только с аргументами",
			},
			&cli.StringFlag{
				Name:  nameFlag,
				Usage: "имя (alias) протокола-кандидата; пусто — протокол по умолчанию; только с аргументами",
			},
			&cli.StringFlag{
				Name:    manifestFlag,
				Aliases: []string{"f"},
				Usage:   "путь к манифесту (по умолчанию paas.toml); только без аргументов",
			},
		},
		Action: c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	switch cmd.Args().Len() {
	case 0:
		return c.runManifest(ctx, cmd)
	case 2:
		return c.runCandidate(ctx, cmd)
	default:
		return fmt.Errorf("укажите <service-id> и путь к файлу кандидата либо запустите без аргументов — проверка всех контрактов манифеста")
	}
}

func (c *Command) runCandidate(ctx context.Context, cmd *cli.Command) error {
	// Формат проверяем до чтения файла и похода на платформу: опечатка — ошибка
	// сразу, а не молчаливая проверка не тем типом (как в publish, CLI-18).
	format, err := entities.ParseProtocolFormat(cmd.String(formatFlag))
	if err != nil {
		return err
	}

	report, err := c.checker.Execute(ctx, usecases.CheckCompatibilityInput{
		ServiceID:     cmd.Args().Get(0),
		Name:          cmd.String(nameFlag),
		Format:        format,
		CandidatePath: cmd.Args().Get(1),
	})
	if err != nil {
		return err
	}

	return render(cmd.Root().Writer, report)
}

func (c *Command) runManifest(ctx context.Context, cmd *cli.Command) error {
	report, err := c.manifest.Execute(ctx, usecases.CheckManifestCompatibilityInput{
		ManifestPath: cmd.String(manifestFlag),
	})
	if err != nil {
		return err
	}

	w := cmd.Root().Writer
	breaking := false
	for _, named := range report.Reports {
		if named.Name != "" {
			fmt.Fprintf(w, "Протокол %q:\n", named.Name)
		}
		if err := render(w, &named.Report); errors.Is(err, errBreaking) {
			// Ломающий кандидат останавливает команду, но сводки по остальным
			// протоколам всё равно печатаются — чинить их разом дешевле.
			breaking = true
		}
	}
	for _, name := range report.Orphaned {
		fmt.Fprintf(w, "Внимание: протокол %q остался в реестре, но исчез из манифеста.\n", name)
	}
	if breaking {
		return errBreaking
	}
	return nil
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
