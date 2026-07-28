package clusterconnectcommandcli

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

type Command struct {
	connect ClusterConnector
}

func New(connect ClusterConnector) *Command {
	return &Command{connect: connect}
}

// CLICommand описывает `clusters connect` для urfave/cli.
func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:      "connect",
		Usage:     "подключить кластер из kubeconfig к платформе",
		ArgsUsage: "",
		Description: "Заводит в кластере учётную запись для платформы вашим же доступом и\n" +
			"регистрирует кластер. Ваш личный доступ платформе не передаётся.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "name",
				Usage:    "имя кластера на платформе (kebab-case)",
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
	name := strings.TrimSpace(cmd.String("name"))
	if name == "" {
		return entities.ErrEmptyClusterName
	}

	out := cmd.Root().Writer
	confirm := c.confirmer(cmd.Bool("yes"), out, cmd.Root().Reader)

	cluster, err := c.connect.Execute(ctx, usecases.ConnectClusterInput{
		Name:       name,
		Context:    cmd.String("context"),
		Kubeconfig: cmd.String("kubeconfig"),
	}, confirm)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "✓ Кластер %s подключён (%s)\n", cluster.Name, cluster.Endpoint)
	if !cluster.Connected {
		// Домен подтверждает связь при подключении, поэтому сюда попасть трудно;
		// но молчать о неподтверждённой связи нельзя — на неё будут ссылаться
		// окружения.
		fmt.Fprintln(out, "  связь пока не подтверждена — проверьте перечень кластеров")
	}
	return nil
}

// confirmer показывает, что именно появится в чужом кластере, и спрашивает
// согласие: молча менять чужой кластер нельзя, даже своей же командой.
func (c *Command) confirmer(skip bool, out interface{ Write([]byte) (int, error) }, in interface{ Read([]byte) (int, error) }) usecases.ConfirmFunc {
	return func(plan usecases.ConnectClusterPlan) (bool, error) {
		fmt.Fprintf(out, "Кластер: %s\n", plan.Endpoint)
		fmt.Fprintf(out, "Будет заведена учётная запись %s и роль с правами:\n", plan.ServiceAccount)
		for _, rule := range plan.Rules {
			groups := strings.Join(rule.APIGroups, ", ")
			if strings.TrimSpace(groups) == "" {
				groups = "core"
			}
			fmt.Fprintf(out, "  • %s: %s → %s\n",
				groups, strings.Join(rule.Resources, ", "), strings.Join(rule.Verbs, ", "))
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
