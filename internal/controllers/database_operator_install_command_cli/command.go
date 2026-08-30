package databaseoperatorinstallcommandcli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

type Command struct {
	install OperatorInstaller
}

func New(install OperatorInstaller) *Command {
	return &Command{install: install}
}

// CLICommand описывает `databases operators install` для urfave/cli.
func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:  "install",
		Usage: "поставить оператор СУБД в подключённый кластер вашим доступом",
		Description: "Применяет манифест оператора, полученный от платформы, и выдаёт учётной\n" +
			"записи платформы право заводить СУБД этого типа. Ваш личный доступ\n" +
			"платформе не передаётся. Повтор обновляет оператор до актуальной версии.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "engine",
				Usage:    "тип СУБД (postgres)",
				Required: true,
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
	confirm := c.confirmer(cmd.Bool("yes"), out, cmd.Root().Reader)

	result, err := c.install.Execute(ctx, usecases.InstallOperatorInput{
		Engine:     strings.TrimSpace(cmd.String("engine")),
		Context:    cmd.String("context"),
		Kubeconfig: cmd.String("kubeconfig"),
	}, confirm)
	if err != nil {
		return err
	}

	report := result.Report
	created, updated := report.Count(entities.ChangeCreated), report.Count(entities.ChangeUpdated)
	switch {
	case created == 0 && updated == 0:
		fmt.Fprintf(out, "✓ Оператор в кластере %s уже актуален — менять нечего\n", result.ClusterName)
	default:
		fmt.Fprintf(out, "✓ Оператор установлен в кластер %s: создано %d, обновлено %d, без изменений %d\n",
			result.ClusterName, created, updated, report.Count(entities.ChangeUnchanged))
		for _, change := range report.Changes {
			if change.Change == entities.ChangeUnchanged {
				continue
			}
			fmt.Fprintf(out, "  %s %s\n", changeMark(change.Change), describe(change.ManifestObject))
		}
	}
	return nil
}

// confirmer показывает, что именно появится в чужом кластере, и спрашивает
// согласие: молча менять чужой кластер нельзя, даже своей же командой.
func (c *Command) confirmer(skip bool, out io.Writer, in io.Reader) usecases.InstallOperatorConfirmFunc {
	return func(plan usecases.InstallOperatorPlan) (bool, error) {
		fmt.Fprintf(out, "Кластер: %s (%s)\n", plan.ClusterName, plan.Endpoint)
		fmt.Fprintf(out, "Оператор: %s %s — будет применено объектов: %d\n", plan.Operator.Name, plan.Operator.Version, len(plan.Objects))
		for _, line := range summarize(plan.Objects) {
			fmt.Fprintf(out, "  • %s\n", line)
		}
		fmt.Fprintf(out, "Учётная запись платформы %s получит право:\n", plan.ServiceAccount)
		for _, rule := range plan.Operator.Rules {
			groups := strings.Join(rule.APIGroups, ", ")
			if strings.TrimSpace(groups) == "" {
				groups = "core"
			}
			fmt.Fprintf(out, "  • %s: %s → %s\n", groups, strings.Join(rule.Resources, ", "), strings.Join(rule.Verbs, ", "))
			if rule.Comment != "" {
				fmt.Fprintf(out, "    %s\n", rule.Comment)
			}
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

// summarize сворачивает объекты по виду: релиз оператора — сотни строк CRD и
// десяток ролей, по одному их читать никто не будет.
func summarize(objects []entities.ManifestObject) []string {
	counts := map[string]int{}
	var order []string
	for _, obj := range objects {
		if _, seen := counts[obj.Kind]; !seen {
			order = append(order, obj.Kind)
		}
		counts[obj.Kind]++
	}
	sort.Strings(order)
	lines := make([]string, 0, len(order))
	for _, kind := range order {
		lines = append(lines, fmt.Sprintf("%s ×%d", kind, counts[kind]))
	}
	return lines
}

func describe(obj entities.ManifestObject) string {
	if obj.Namespace == "" {
		return fmt.Sprintf("%s %s", obj.Kind, obj.Name)
	}
	return fmt.Sprintf("%s %s/%s", obj.Kind, obj.Namespace, obj.Name)
}

func changeMark(change entities.Change) string {
	if change == entities.ChangeCreated {
		return "+"
	}
	return "~"
}
