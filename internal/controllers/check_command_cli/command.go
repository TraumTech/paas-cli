// Package checkcommandcli — входной адаптер: команда `check` (TRA-556).
// Проверяет файл сервиса до пуша и ничего не меняет на платформе.
package checkcommandcli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

const (
	fileFlag        = "file"
	defaultFilePath = "paas.toml"
)

type Command struct {
	checker ServiceFileChecker
}

func New(checker ServiceFileChecker) *Command {
	return &Command{checker: checker}
}

func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:  "check",
		Usage: "проверить файл сервиса до пуша: формат и, после входа, сверка с организацией",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    fileFlag,
				Aliases: []string{"f"},
				Value:   defaultFilePath,
				Usage:   "путь к файлу сервиса",
			},
		},
		Action: c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	result, err := c.checker.Execute(ctx, usecases.CheckServiceFileInput{Path: cmd.String(fileFlag)})
	if err != nil {
		return err
	}
	out := cmd.Root().Writer
	if len(result.Missing) > 0 {
		message := fmt.Sprintf("%s: формат верный, но файл ссылается на то, чего нет в организации (%d)", result.Path, len(result.Missing))
		for _, m := range result.Missing {
			message += "\n  " + m.String()
		}
		return fmt.Errorf("%s", message)
	}
	if result.PlatformSkipped {
		fmt.Fprintf(out, "✓ %s: файл в порядке (сервис %s)\n", result.Path, result.Service)
		fmt.Fprintln(out, "  Сверка с платформой пропущена: вход не выполнен. Войдите — `stapel auth login` — и команда проверит, что сервис, окружения, зоны и СУБД из файла заведены в организации.")
	} else {
		fmt.Fprintf(out, "✓ %s: файл в порядке (сервис %s); всё, на что он ссылается, заведено в организации\n", result.Path, result.Service)
	}
	for _, note := range result.Notes {
		fmt.Fprintln(out, "  Замечание: "+note)
	}
	return nil
}
