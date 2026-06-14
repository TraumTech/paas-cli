package app

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/TraumTech/paas-cli/internal/adapters/protocol_source_http"
	"github.com/TraumTech/paas-cli/internal/adapters/protocol_store_file"
	"github.com/TraumTech/paas-cli/internal/controllers/protocol_fetch_command_cli"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

const (
	defaultAPIURL      = "https://api.paas.traumtech.ru"
	defaultDestination = "protocols"
	httpTimeout        = 30 * time.Second
)

// Run собирает корневую команду CLI и запускает её. Адрес платформы берётся из
// PAAS_API_URL (по умолчанию прод), поэтому команда работает из любого
// репозитория-потребителя.
func Run(ctx context.Context, args []string) error {
	baseURL := strings.TrimRight(envOr("PAAS_API_URL", defaultAPIURL), "/")
	source := protocolsourcehttp.New(baseURL, &http.Client{Timeout: httpTimeout})
	store := protocolstorefile.New()
	fetch := protocolfetchcommandcli.New(usecases.NewFetchProtocol(source, store))

	root := &cli.Command{
		Name:  "paas-cli",
		Usage: "получение контрактов сервисов платформы",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    protocolfetchcommandcli.DestinationFlag,
				Aliases: []string{"d"},
				Value:   defaultDestination,
				Usage:   "директория для контрактов (файл: <dest>/<service-name>/openapi.json)",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "protocols",
				Usage: "работа с контрактами (протоколами) сервисов",
				Commands: []*cli.Command{
					fetch.CLICommand(),
				},
			},
		},
	}

	return root.Run(ctx, args)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
