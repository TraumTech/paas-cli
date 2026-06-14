package protocolfetchcommandcli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

// DestinationFlag — имя глобального флага с директорией для контрактов; команда
// читает его из родительской команды.
const DestinationFlag = "destination"

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
		Action:    c.run,
	}
}

func (c *Command) run(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return fmt.Errorf("нужно указать ровно один <service-id>")
	}

	result, err := c.fetcher.Execute(ctx, usecases.FetchProtocolInput{
		ServiceID:   cmd.Args().First(),
		Destination: cmd.String(DestinationFlag),
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.Root().Writer, "✓ Контракт сервиса %s (версия %d) записан в %s\n",
		result.ServiceName, result.VersionNumber, result.Path)
	return nil
}
