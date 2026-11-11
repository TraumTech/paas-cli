package databaseforwardcommandcli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

const defaultPort = 15432

type DatabaseForwarder interface {
	Execute(ctx context.Context, input usecases.ForwardDatabaseInput, confirm usecases.ForwardDatabaseConfirmFunc, events usecases.ForwardDatabaseEvents) error
}

type Command struct {
	forward DatabaseForwarder
}

func New(forward DatabaseForwarder) *Command {
	return &Command{forward: forward}
}

// CLICommand описывает `databases forward` для urfave/cli.
func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:      "forward",
		Usage:     "держать локальный порт на СУБД в кластере вашим доступом — для DataGrip, psql и других клиентов",
		ArgsUsage: "<СУБД>",
		Description: "Находит СУБД CloudNativePG по имени, при первом запуске заводит в ней вашу\n" +
			"личную роль только на чтение (pg_read_all_data) и держит 127.0.0.1:<port>\n" +
			"на основном инстансе через API кластера, переподключаясь после обрывов.\n" +
			"Наружу у СУБД ничего не открывается.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "namespace",
				Usage: "namespace СУБД; по умолчанию ищется во всём кластере",
			},
			&cli.StringFlag{
				Name:  "role",
				Value: defaultRole(),
				Usage: "имя личной роли в СУБД; по умолчанию имя пользователя ОС",
			},
			&cli.IntFlag{
				Name:  "port",
				Value: defaultPort,
				Usage: "локальный порт; заведите по своему на каждую СУБД, чтобы не перенастраивать клиент",
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
				Usage: "не спрашивать подтверждение, если роль нужно завести",
			},
		},
		Action: c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	out := cmd.Root().Writer
	errOut := cmd.Root().ErrWriter
	if errOut == nil {
		errOut = os.Stderr
	}
	return c.forward.Execute(ctx, usecases.ForwardDatabaseInput{
		Server:     cmd.Args().First(),
		Namespace:  cmd.String("namespace"),
		Role:       strings.TrimSpace(cmd.String("role")),
		Port:       int(cmd.Int("port")),
		Context:    cmd.String("context"),
		Kubeconfig: cmd.String("kubeconfig"),
	}, confirmer(cmd.Bool("yes"), out, cmd.Root().Reader), events(out, errOut))
}

// confirmer спрашивает согласие на правку CR СУБД: молча менять кластер
// нельзя, даже своей же командой.
func confirmer(skip bool, out io.Writer, in io.Reader) usecases.ForwardDatabaseConfirmFunc {
	return func(plan usecases.ForwardDatabasePlan) (bool, error) {
		fmt.Fprintf(out, "Контекст: %s\n", plan.ContextName)
		fmt.Fprintf(out, "В СУБД %s/%s будет заведена роль %s:\n", plan.Server.Namespace, plan.Server.Name, plan.Role)
		fmt.Fprintf(out, "  • Secret %s/paas-user-%s с паролем\n", plan.Server.Namespace, strings.ReplaceAll(plan.Role, "_", "-"))
		fmt.Fprintf(out, "  • роль в spec.managed.roles: вход, pg_read_all_data, pg_monitor\n")
		if skip {
			return true, nil
		}
		fmt.Fprint(out, "Завести роль? [y/N]: ")
		answer, err := bufio.NewReader(in).ReadString('\n')
		if err != nil {
			return false, fmt.Errorf("прочитать ответ: %w", err)
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		return answer == "y" || answer == "yes" || answer == "д" || answer == "да", nil
	}
}

func events(out, errOut io.Writer) usecases.ForwardDatabaseEvents {
	return usecases.ForwardDatabaseEvents{
		Ready: func(s usecases.ForwardDatabaseSession) {
			fmt.Fprintf(out, "✓ СУБД %s/%s доступна на 127.0.0.1:%d — держите команду запущенной, Ctrl+C останавливает\n\n",
				s.Server.Namespace, s.Server.Name, s.Port)
			fmt.Fprintf(out, "  Хост:     127.0.0.1\n  Порт:     %d\n  Роль:     %s\n  Пароль:   %s\n  SSL:      require\n",
				s.Port, s.Credentials.User, s.Credentials.Password)
			databases := "—"
			if len(s.Server.Databases) > 0 {
				databases = strings.Join(s.Server.Databases, ", ")
			}
			fmt.Fprintf(out, "  Базы:     %s\n\n", databases)
			fmt.Fprintf(out, "  postgresql://%s@127.0.0.1:%d/postgres?sslmode=require\n", s.Credentials.User, s.Port)
		},
		Lost: func(err error, retryIn time.Duration) {
			fmt.Fprintf(errOut, "⟳ связь с СУБД потеряна (%v) — переподключаюсь через %s\n", err, retryIn)
		},
		Reconnected: func() {
			fmt.Fprintf(errOut, "✓ снова на связи\n")
		},
	}
}

var nonRoleChars = regexp.MustCompile(`[^a-z0-9_]`)

func defaultRole() string {
	return nonRoleChars.ReplaceAllString(strings.ToLower(os.Getenv("USER")), "_")
}
