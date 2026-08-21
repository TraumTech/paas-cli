package protocolfetchcommandcli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

// DestinationFlag — имя глобального флага с директорией для контрактов; команда
// читает его из родительской команды.
const DestinationFlag = "destination"

// methodFlag — локальный флаг выбора методов; повторяемый или через запятую.
const methodFlag = "method"

// attributeFlag — локальный флаг среза до атрибутов внутри выбранных методов
// (PRT-29); повторяемый или через запятую.
const attributeFlag = "attribute"

type Command struct {
	fetcher ProtocolFetcher
}

func New(fetcher ProtocolFetcher) *Command {
	return &Command{fetcher: fetcher}
}

// CLICommand описывает подкоманду `fetch` для urfave/cli.
func (c *Command) CLICommand() *cli.Command {
	return &cli.Command{
		Name:      "fetch",
		Usage:     "получить актуальный опубликованный контракт сервиса",
		ArgsUsage: "<service-id>",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:    methodFlag,
				Aliases: []string{"m"},
				Usage:   `оставить в контракте только указанные методы: HTTP-паттерн ("GET /services/{id}") у OpenAPI, package.Service/Method у gRPC; можно повторять или через запятую`,
			},
			&cli.StringSliceFlag{
				Name:    attributeFlag,
				Aliases: []string{"a"},
				Usage:   `оставить внутри выбранных методов только указанные атрибуты (идентичностью атрибута: "GET /services#response.200.name" у OpenAPI); требует --method; можно повторять или через запятую`,
			},
		},
		Action: c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("нужно указать ровно один <service-id>")
	}

	var methods []string
	if m := cmd.StringSlice(methodFlag); len(m) > 0 {
		methods = m
	}
	var attributes []string
	if a := cmd.StringSlice(attributeFlag); len(a) > 0 {
		attributes = a
	}
	if len(attributes) > 0 && len(methods) == 0 {
		return fmt.Errorf("--%s требует --%s: атрибуты объявляются внутри выбранных методов", attributeFlag, methodFlag)
	}
	result, err := c.fetcher.Execute(ctx, usecases.FetchProtocolInput{
		ServiceID:   cmd.Args().First(),
		Destination: cmd.String(DestinationFlag),
		Methods:     methods,
		Attributes:  attributes,
	})
	if err != nil {
		return err
	}

	if len(attributes) > 0 {
		fmt.Fprintf(cmd.Root().Writer, "✓ Контракт сервиса %s (версия %d, частичный: %d метод(ов), %d атрибут(ов)) записан в %s\n",
			result.ServiceName, result.VersionNumber, len(methods), len(attributes), result.Path)
		return nil
	}
	if len(methods) > 0 {
		fmt.Fprintf(cmd.Root().Writer, "✓ Контракт сервиса %s (версия %d, частичный: %d метод(ов)) записан в %s\n",
			result.ServiceName, result.VersionNumber, len(methods), result.Path)
		return nil
	}
	fmt.Fprintf(cmd.Root().Writer, "✓ Контракт сервиса %s (версия %d%s) записан в %s\n",
		result.ServiceName, result.VersionNumber, formatLabel(result.Format), result.Path)
	return nil
}

// formatLabel — пометка формата в отчёте; прежний формат (OpenAPI) не помечаем,
// чтобы привычный вывод не менялся.
func formatLabel(f entities.ProtocolFormat) string {
	if f == entities.ProtocolFormatGRPC {
		return ", gRPC"
	}
	return ""
}
