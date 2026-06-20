package app

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/TraumTech/paas-cli/internal/adapters/candidate_reader_file"
	"github.com/TraumTech/paas-cli/internal/adapters/dependency_registrar_http"
	"github.com/TraumTech/paas-cli/internal/adapters/manifest_reader_file"
	"github.com/TraumTech/paas-cli/internal/adapters/protocol_compatibility_http"
	"github.com/TraumTech/paas-cli/internal/adapters/protocol_publish_http"
	"github.com/TraumTech/paas-cli/internal/adapters/protocol_source_http"
	"github.com/TraumTech/paas-cli/internal/adapters/protocol_store_file"
	"github.com/TraumTech/paas-cli/internal/adapters/service_resolver_http"
	"github.com/TraumTech/paas-cli/internal/adapters/version_publisher_http"
	"github.com/TraumTech/paas-cli/internal/controllers/dependency_register_command_cli"
	"github.com/TraumTech/paas-cli/internal/controllers/protocol_compatibility_command_cli"
	"github.com/TraumTech/paas-cli/internal/controllers/protocol_fetch_command_cli"
	"github.com/TraumTech/paas-cli/internal/controllers/protocol_publish_command_cli"
	"github.com/TraumTech/paas-cli/internal/controllers/protocol_sync_command_cli"
	"github.com/TraumTech/paas-cli/internal/controllers/version_publish_command_cli"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

const (
	defaultAPIURL      = "https://api.paas.traumtech.ru"
	defaultDestination = "protocols"
	httpTimeout        = 30 * time.Second
	// envAPIToken — машинный креденшел сервиса для неинтерактивного доступа (CI, скрипты).
	envAPIToken = "PAAS_API_TOKEN"
)

// Version — версия бинаря; подставляется при сборке релиза (GoReleaser, ldflags).
var Version = "dev"

// Run собирает корневую команду CLI и запускает её. Адрес платформы берётся из
// PAAS_API_URL (по умолчанию прод), поэтому команда работает из любого
// репозитория-потребителя.
func Run(ctx context.Context, args []string) error {
	baseURL := strings.TrimRight(envOr("PAAS_API_URL", defaultAPIURL), "/")
	// Машинный креденшел сервиса (если задан) уходит со всеми запросами к платформе.
	client := httpClient(os.Getenv(envAPIToken))

	source, err := protocolsourcehttp.New(baseURL, client)
	if err != nil {
		return err
	}
	store := protocolstorefile.New()
	fetch := protocolfetchcommandcli.New(usecases.NewFetchProtocol(source, store))

	resolver, err := serviceresolverhttp.New(baseURL, client)
	if err != nil {
		return err
	}
	manifests := manifestreaderfile.New()
	sync := protocolsynccommandcli.New(usecases.NewSyncProtocols(manifests, resolver, source, store))

	compatSource, err := protocolcompatibilityhttp.New(baseURL, client)
	if err != nil {
		return err
	}
	candidates := candidatereaderfile.New()
	compat := protocolcompatibilitycommandcli.New(usecases.NewCheckCompatibility(candidates, compatSource))

	publisher, err := versionpublisherhttp.New(baseURL, client)
	if err != nil {
		return err
	}
	publishVersion := versionpublishcommandcli.New(usecases.NewPublishVersion(manifests, resolver, publisher))

	publishSource, err := protocolpublishhttp.New(baseURL, client)
	if err != nil {
		return err
	}
	publish := protocolpublishcommandcli.New(usecases.NewPublishProtocol(manifests, resolver, candidates, publishSource))

	registrar, err := dependencyregistrarhttp.New(baseURL, client)
	if err != nil {
		return err
	}
	registerDependency := dependencyregistercommandcli.New(usecases.NewRegisterDependency(manifests, resolver, candidates, registrar))

	root := &cli.Command{
		Name:    "paas-cli",
		Usage:   "получение контрактов сервисов платформы",
		Version: Version,
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
					sync.CLICommand(),
					compat.CLICommand(),
					publish.CLICommand(),
				},
			},
			{
				Name:  "versions",
				Usage: "работа с версиями сервисов",
				Commands: []*cli.Command{
					publishVersion.CLICommand(),
				},
			},
			{
				Name:  "dependencies",
				Usage: "зависимости версий потребителя от контрактов продьюсеров",
				Commands: []*cli.Command{
					registerDependency.CLICommand(),
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
