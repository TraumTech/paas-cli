package app

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/TraumTech/paas-observability-sdk/sdk/observabilityhttp"
	"github.com/urfave/cli/v3"

	"github.com/TraumTech/paas-cli/internal/adapters/candidate_reader_file"
	"github.com/TraumTech/paas-cli/internal/adapters/dependency_registrar_http"
	"github.com/TraumTech/paas-cli/internal/adapters/manifest_reader_file"
	"github.com/TraumTech/paas-cli/internal/adapters/protocol_compatibility_http"
	"github.com/TraumTech/paas-cli/internal/adapters/protocol_publish_http"
	"github.com/TraumTech/paas-cli/internal/adapters/protocol_source_http"
	"github.com/TraumTech/paas-cli/internal/adapters/protocol_store_file"
	"github.com/TraumTech/paas-cli/internal/adapters/service_resolver_http"
	"github.com/TraumTech/paas-cli/internal/adapters/session_gateway_http"
	"github.com/TraumTech/paas-cli/internal/adapters/session_store_file"
	"github.com/TraumTech/paas-cli/internal/adapters/version_publisher_http"
	"github.com/TraumTech/paas-cli/internal/controllers/auth_login_command_cli"
	"github.com/TraumTech/paas-cli/internal/controllers/auth_logout_command_cli"
	"github.com/TraumTech/paas-cli/internal/controllers/auth_whoami_command_cli"
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
	defaultAuthURL     = "https://auth.paas.traumtech.ru"
	defaultDestination = "protocols"
	httpTimeout        = 30 * time.Second
	// obsFlushTimeout ограничивает досылку трасс при завершении: без сети
	// команда завершается с этой задержкой максимум, а не висит.
	obsFlushTimeout = 3 * time.Second
	// envAPIToken — машинный креденшел сервиса для неинтерактивного доступа (CI, скрипты).
	envAPIToken = "PAAS_API_TOKEN"
	// envAuthURL — адрес identity-провайдера платформы для входа пользователя.
	envAuthURL = "PAAS_AUTH_URL"
)

// Version — версия бинаря; подставляется при сборке релиза (GoReleaser, ldflags).
var Version = "dev"

// Run собирает корневую команду CLI и запускает её. Адрес платформы берётся из
// PAAS_API_URL (по умолчанию прод), поэтому команда работает из любого
// репозитория-потребителя.
func Run(ctx context.Context, args []string) error {
	obs, obsFlush := newObserver(Version)
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), obsFlushTimeout)
		defer cancel()
		_ = obsFlush(flushCtx)
	}()

	baseURL := strings.TrimRight(envOr("PAAS_API_URL", defaultAPIURL), "/")
	sessions := sessionstorefile.New()
	// Машинный креденшел сервиса (если задан) уходит со всеми запросами к платформе;
	// без него — локально сохранённый вход пользователя (auth login), если он есть.
	serviceToken := os.Getenv(envAPIToken)
	sessionToken := ""
	if serviceToken == "" {
		if t, err := sessions.Load(ctx); err == nil {
			sessionToken = t
		}
	}
	client := httpClient(obs, serviceToken, sessionToken)

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

	// Identity-провайдер — отдельный хост со своим клиентом: креденшелы платформы
	// (bearer/сессия) к нему не прикладываются.
	authURL := strings.TrimRight(envOr(envAuthURL, defaultAuthURL), "/")
	gateway := sessiongatewayhttp.New(authURL, &http.Client{
		Timeout:   httpTimeout,
		Transport: observabilityhttp.NewTransport(obs, nil),
	})
	login := authlogincommandcli.New(usecases.NewLogin(gateway, sessions))
	whoami := authwhoamicommandcli.New(usecases.NewWhoAmI(sessions, gateway))
	logout := authlogoutcommandcli.New(usecases.NewLogout(sessions, gateway))

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
			{
				Name:  "auth",
				Usage: "вход в CLI под своей учётной записью платформы",
				Commands: []*cli.Command{
					login.CLICommand(),
					whoami.CLICommand(),
					logout.CLICommand(),
				},
			},
		},
	}

	instrumentCommands(obs, root.Commands, "")

	return root.Run(ctx, args)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
