package clusterlogsinstallcommandcli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

// KeyEnv — ключ приёма логов из окружения: чтобы не оставлять его в истории
// команд и не заводить файл.
const KeyEnv = "STAPEL_LOGS_KEY"

type Command struct {
	install LogCollectorInstaller
}

func New(install LogCollectorInstaller) *Command {
	return &Command{install: install}
}

// CLICommand описывает `clusters logs install` для urfave/cli.
func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:  "install",
		Usage: "поставить в кластер сборщик вывода сервисов вашим доступом",
		Description: "Ставит в кластер сборщик, который читает вывод подов на нодах и шлёт его\n" +
			"платформе: оттуда его показывает вкладка «Выкатки». Сам кластер платформе\n" +
			"для этого не открывается. Повтор обновляет сборщик и ключ.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "key-file",
				Usage: "файл с ключом приёма логов этого кластера; иначе ключ берётся из " + KeyEnv,
			},
			&cli.StringFlag{
				Name:  "endpoint",
				Usage: "адрес приёма логов",
				Value: entities.DefaultLogPushURL,
			},
			&cli.StringFlag{
				Name:  "context",
				Usage: "контекст kubeconfig; по умолчанию текущий",
			},
			&cli.StringFlag{
				Name:  "kubeconfig",
				Usage: "путь к kubeconfig; по умолчанию KUBECONFIG или ~/.kube/config",
			},
			&cli.BoolFlag{
				Name:  "yes",
				Usage: "не спрашивать подтверждение (для автоматизации)",
			},
		},
		Action: c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	out := cmd.Root().Writer
	key, err := readKey(cmd.String("key-file"))
	if err != nil {
		return err
	}

	result, err := c.install.Execute(ctx, usecases.InstallLogCollectorInput{
		Key:        key,
		PushURL:    cmd.String("endpoint"),
		Context:    cmd.String("context"),
		Kubeconfig: cmd.String("kubeconfig"),
	}, confirmer(cmd.Bool("yes"), out, cmd.Root().Reader))
	if err != nil {
		return err
	}

	report := result.Report
	created, updated := report.Count(entities.ChangeCreated), report.Count(entities.ChangeUpdated)
	if created == 0 && updated == 0 {
		fmt.Fprintf(out, "✓ Сборщик логов в кластере %s уже актуален — менять нечего\n", result.Endpoint)
		return nil
	}
	fmt.Fprintf(out, "✓ Сборщик логов работает в кластере %s: создано %d, обновлено %d, без изменений %d\n",
		result.Endpoint, created, updated, report.Count(entities.ChangeUnchanged))
	return nil
}

func readKey(path string) (string, error) {
	if path == "" {
		return os.Getenv(KeyEnv), nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("прочитать файл с ключом: %w", err)
	}
	return string(raw), nil
}

// confirmer показывает, что появится в кластере, и спрашивает согласие.
func confirmer(skip bool, out io.Writer, in io.Reader) usecases.InstallLogCollectorConfirmFunc {
	return func(plan usecases.InstallLogCollectorPlan) (bool, error) {
		fmt.Fprintf(out, "Кластер: %s\n", plan.Endpoint)
		fmt.Fprintf(out, "Вывод подов всех namespace будет уходить на %s\n", plan.PushURL)
		fmt.Fprintf(out, "Будет применено объектов: %d\n", len(plan.Objects))
		for _, obj := range plan.Objects {
			if obj.Namespace == "" {
				fmt.Fprintf(out, "  • %s %s\n", obj.Kind, obj.Name)
				continue
			}
			fmt.Fprintf(out, "  • %s %s/%s\n", obj.Kind, obj.Namespace, obj.Name)
		}
		if skip {
			return true, nil
		}
		fmt.Fprint(out, "Применить в кластере? [y/N]: ")
		answer, err := bufio.NewReader(in).ReadString('\n')
		if err != nil {
			return false, fmt.Errorf("прочитать ответ: %w", err)
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		return answer == "y" || answer == "yes" || answer == "д" || answer == "да", nil
	}
}
