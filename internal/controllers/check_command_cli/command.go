// Package checkcommandcli — входной адаптер: команда `check` (TRA-556).
// Проверяет файл сервиса до пуша и ничего не меняет на платформе.
package checkcommandcli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

const fileFlag = "file"

type Command struct {
	checker ServiceFileChecker
	// locate — файл сервиса текущего каталога, когда путь не задан.
	locate func() string
}

func New(checker ServiceFileChecker, locate func() string) *Command {
	return &Command{checker: checker, locate: locate}
}

func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:  "check",
		Usage: "проверить файл сервиса до пуша: формат и, после входа, сверка с организацией",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    fileFlag,
				Aliases: []string{"f"},
				Usage:   "путь к файлу сервиса (по умолчанию stapel.toml, при его отсутствии paas.toml)",
			},
		},
		Action: c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	path := cmd.String(fileFlag)
	if path == "" {
		path = c.locate()
	}
	result, err := c.checker.Execute(ctx, usecases.CheckServiceFileInput{Path: path})
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
