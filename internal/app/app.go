package app

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/TraumTech/paas-observability-sdk/sdk/observabilityhttp"
	"github.com/urfave/cli/v3"

	browserauthorizerlocal "github.com/TraumTech/paas-cli/internal/adapters/browser_authorizer_local"
	"github.com/TraumTech/paas-cli/internal/adapters/candidate_reader_file"
	clusteraccesshttp "github.com/TraumTech/paas-cli/internal/adapters/cluster_access_http"
	clusterprovisionerk8s "github.com/TraumTech/paas-cli/internal/adapters/cluster_provisioner_k8s"
	clusterregistrarhttp "github.com/TraumTech/paas-cli/internal/adapters/cluster_registrar_http"
	"github.com/TraumTech/paas-cli/internal/adapters/dependency_registrar_http"
	formreaderfile "github.com/TraumTech/paas-cli/internal/adapters/form_reader_file"
	"github.com/TraumTech/paas-cli/internal/adapters/manifest_reader_file"
	personaltokenhttp "github.com/TraumTech/paas-cli/internal/adapters/personal_token_http"
	"github.com/TraumTech/paas-cli/internal/adapters/protocol_compatibility_http"
	"github.com/TraumTech/paas-cli/internal/adapters/protocol_publish_http"
	"github.com/TraumTech/paas-cli/internal/adapters/registry_directory_http"
	"github.com/TraumTech/paas-cli/internal/adapters/protocol_source_http"
	"github.com/TraumTech/paas-cli/internal/adapters/protocol_store_file"
	"github.com/TraumTech/paas-cli/internal/adapters/service_resolver_http"
	"github.com/TraumTech/paas-cli/internal/adapters/session_gateway_http"
	"github.com/TraumTech/paas-cli/internal/adapters/session_store_file"
	"github.com/TraumTech/paas-cli/internal/adapters/version_publisher_http"
	"github.com/TraumTech/paas-cli/internal/controllers/auth_login_command_cli"
	"github.com/TraumTech/paas-cli/internal/controllers/auth_logout_command_cli"
	"github.com/TraumTech/paas-cli/internal/controllers/auth_whoami_command_cli"
	buildpublishcommandcli "github.com/TraumTech/paas-cli/internal/controllers/build_publish_command_cli"
	clusterconnectcommandcli "github.com/TraumTech/paas-cli/internal/controllers/cluster_connect_command_cli"
	"github.com/TraumTech/paas-cli/internal/controllers/dependency_register_command_cli"
	"github.com/TraumTech/paas-cli/internal/controllers/protocol_compatibility_command_cli"
	"github.com/TraumTech/paas-cli/internal/controllers/protocol_fetch_command_cli"
	"github.com/TraumTech/paas-cli/internal/controllers/protocol_publish_command_cli"
	"github.com/TraumTech/paas-cli/internal/controllers/protocol_sync_command_cli"
	"github.com/TraumTech/paas-cli/internal/controllers/version_publish_command_cli"
	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

const (
	defaultAPIURL      = "https://api.paas.traumtech.ru"
	defaultAuthURL     = "https://auth.paas.traumtech.ru"
	defaultWebURL      = "https://paas.traumtech.ru"
	defaultDestination = "protocols"
	httpTimeout        = 30 * time.Second
	// obsFlushTimeout ограничивает досылку трасс при завершении: без сети
	// команда завершается с этой задержкой максимум, а не висит.
	obsFlushTimeout = 3 * time.Second
	// envAPIToken — машинный креденшел сервиса для неинтерактивного доступа (CI, скрипты).
	envAPIToken = "PAAS_API_TOKEN"
	// envAuthURL — адрес identity-провайдера платформы для входа пользователя.
	envAuthURL = "PAAS_AUTH_URL"
	// envWebURL — адрес интерфейса платформы: там живёт страница подтверждения
	// браузерного входа (AUTH-22).
	envWebURL = "PAAS_WEB_URL"
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
	// Токен из окружения (если задан) уходит со всеми запросами к платформе;
	// без него — локально сохранённый вход пользователя (auth login), если он есть.
	envToken := os.Getenv(envAPIToken)
	var credential *entities.Credential
	if envToken == "" {
		if saved, err := sessions.Load(ctx); err == nil {
			credential = saved
		}
	}
	client := httpClient(obs, envToken, credential)

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
	registry, err := registrydirectoryhttp.New(baseURL, client)
	if err != nil {
		return err
	}
	candidates := candidatereaderfile.New()
	compat := protocolcompatibilitycommandcli.New(
		usecases.NewCheckCompatibility(candidates, compatSource),
		usecases.NewCheckManifestCompatibility(manifests, resolver, candidates, compatSource, registry))

	publisher, err := versionpublisherhttp.New(baseURL, client)
	if err != nil {
		return err
	}
	publishVersion := versionpublishcommandcli.New(usecases.NewPublishVersion(manifests, formreaderfile.New(), resolver, publisher))
	publishBuild := buildpublishcommandcli.New(usecases.NewPublishBuild(manifests, formreaderfile.New(), resolver, publisher))

	publishSource, err := protocolpublishhttp.New(baseURL, client)
	if err != nil {
		return err
	}
	publish := protocolpublishcommandcli.New(usecases.NewPublishProtocol(manifests, resolver, candidates, publishSource, registry))

	registrar, err := dependencyregistrarhttp.New(baseURL, client)
	if err != nil {
		return err
	}
	registerDependency := dependencyregistercommandcli.New(usecases.NewRegisterDependency(manifests, resolver, candidates, registrar))

	clusterAccess, err := clusteraccesshttp.New(baseURL, client)
	if err != nil {
		return err
	}
	clusterRegistrar, err := clusterregistrarhttp.New(baseURL, client)
	if err != nil {
		return err
	}
	connectCluster := clusterconnectcommandcli.New(
		usecases.NewConnectCluster(clusterAccess, clusterprovisionerk8s.New(), clusterRegistrar),
	)

	// Identity-провайдер — отдельный хост со своим клиентом: креденшелы платформы
	// (bearer/сессия) к нему не прикладываются.
	authURL := strings.TrimRight(envOr(envAuthURL, defaultAuthURL), "/")
	gateway := sessiongatewayhttp.New(authURL, &http.Client{
		Timeout:   httpTimeout,
		Transport: observabilityhttp.NewTransport(obs, nil),
	})
	personalTokens, err := personaltokenhttp.New(baseURL, client)
	if err != nil {
		return err
	}
	// Страница подтверждения живёт в интерфейсе платформы, где пользователь уже
	// вошёл, — отсюда отдельный адрес.
	webURL := strings.TrimRight(envOr(envWebURL, defaultWebURL), "/")
	authorizer := browserauthorizerlocal.New(webURL, os.Stderr)
	login := authlogincommandcli.New(
		usecases.NewLogin(gateway, sessions),
		usecases.NewBrowserLogin(authorizer, sessions),
	)
	whoami := authwhoamicommandcli.New(usecases.NewWhoAmI(sessions, gateway, personalTokens))
	logout := authlogoutcommandcli.New(usecases.NewLogout(sessions, gateway, personalTokens))

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
				Name:  "builds",
				Usage: "сборки веток сервиса (артефакты выкатки)",
				Commands: []*cli.Command{
					publishBuild.CLICommand(),
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
				Name:  "clusters",
				Usage: "подключение кластеров Kubernetes организации",
				Commands: []*cli.Command{
					connectCluster.CLICommand(),
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
