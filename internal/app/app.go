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
	candidatereaderfile "github.com/TraumTech/paas-cli/internal/adapters/candidate_reader_file"
	clusteraccesshttp "github.com/TraumTech/paas-cli/internal/adapters/cluster_access_http"
	clusterdirectoryhttp "github.com/TraumTech/paas-cli/internal/adapters/cluster_directory_http"
	clusterprovisionerk8s "github.com/TraumTech/paas-cli/internal/adapters/cluster_provisioner_k8s"
	clusterregistrarhttp "github.com/TraumTech/paas-cli/internal/adapters/cluster_registrar_http"
	databaseoperatorhttp "github.com/TraumTech/paas-cli/internal/adapters/database_operator_http"
	dependencyregistrarhttp "github.com/TraumTech/paas-cli/internal/adapters/dependency_registrar_http"
	formreaderfile "github.com/TraumTech/paas-cli/internal/adapters/form_reader_file"
	manifestreaderfile "github.com/TraumTech/paas-cli/internal/adapters/manifest_reader_file"
	oauthclienthydra "github.com/TraumTech/paas-cli/internal/adapters/oauth_client_hydra"
	operatorinstallerk8s "github.com/TraumTech/paas-cli/internal/adapters/operator_installer_k8s"
	protocolcompatibilityhttp "github.com/TraumTech/paas-cli/internal/adapters/protocol_compatibility_http"
	protocolpublishhttp "github.com/TraumTech/paas-cli/internal/adapters/protocol_publish_http"
	protocolsourcehttp "github.com/TraumTech/paas-cli/internal/adapters/protocol_source_http"
	protocolstorefile "github.com/TraumTech/paas-cli/internal/adapters/protocol_store_file"
	registrydirectoryhttp "github.com/TraumTech/paas-cli/internal/adapters/registry_directory_http"
	serviceresolverhttp "github.com/TraumTech/paas-cli/internal/adapters/service_resolver_http"
	sessionstorefile "github.com/TraumTech/paas-cli/internal/adapters/session_store_file"
	userinfohydra "github.com/TraumTech/paas-cli/internal/adapters/user_info_hydra"
	versionpublisherhttp "github.com/TraumTech/paas-cli/internal/adapters/version_publisher_http"
	authlogincommandcli "github.com/TraumTech/paas-cli/internal/controllers/auth_login_command_cli"
	authlogoutcommandcli "github.com/TraumTech/paas-cli/internal/controllers/auth_logout_command_cli"
	authwhoamicommandcli "github.com/TraumTech/paas-cli/internal/controllers/auth_whoami_command_cli"
	buildpublishcommandcli "github.com/TraumTech/paas-cli/internal/controllers/build_publish_command_cli"
	clusterconnectcommandcli "github.com/TraumTech/paas-cli/internal/controllers/cluster_connect_command_cli"
	databaseoperatorinstallcommandcli "github.com/TraumTech/paas-cli/internal/controllers/database_operator_install_command_cli"
	dependencyregistercommandcli "github.com/TraumTech/paas-cli/internal/controllers/dependency_register_command_cli"
	protocolcompatibilitycommandcli "github.com/TraumTech/paas-cli/internal/controllers/protocol_compatibility_command_cli"
	protocolfetchcommandcli "github.com/TraumTech/paas-cli/internal/controllers/protocol_fetch_command_cli"
	protocolpublishcommandcli "github.com/TraumTech/paas-cli/internal/controllers/protocol_publish_command_cli"
	protocolsynccommandcli "github.com/TraumTech/paas-cli/internal/controllers/protocol_sync_command_cli"
	versionpublishcommandcli "github.com/TraumTech/paas-cli/internal/controllers/version_publish_command_cli"
	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

const (
	defaultAPIURL   = "https://api.paas.traumtech.ru"
	defaultOAuthURL = "https://oauth.paas.traumtech.ru"
	// oauthClientID — публичный OAuth-клиент CLI у сервера авторизации платформы
	// (IAP-03); регистрируется в paas-kratos (deploy/hydra-clients.sh).
	oauthClientID      = "paas-cli"
	defaultDestination = "protocols"
	httpTimeout        = 30 * time.Second
	// obsFlushTimeout ограничивает досылку трасс при завершении: без сети
	// команда завершается с этой задержкой максимум, а не висит.
	obsFlushTimeout = 3 * time.Second
	// envAPIToken — машинный креденшел сервиса для неинтерактивного доступа (CI, скрипты).
	envAPIToken = "PAAS_API_TOKEN"
	// envOAuthURL — адрес сервера авторизации платформы (issuer): вход через
	// браузер, продление и отзыв входа (IAP-03).
	envOAuthURL = "PAAS_OAUTH_URL"
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
	// Сервер авторизации — отдельный хост и клиент без креденшелов платформы:
	// код, refresh и отзыв он принимает от публичного клиента сами по себе.
	oauthURL := strings.TrimRight(envOr(envOAuthURL, defaultOAuthURL), "/")
	oauth := oauthclienthydra.New(oauthURL, oauthClientID, &http.Client{
		Timeout:   httpTimeout,
		Transport: observabilityhttp.NewTransport(obs, nil),
	})
	client := httpClient(obs, envToken, credential, oauth, sessions)

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
	publishBuild := buildpublishcommandcli.New(usecases.NewPublishBuild(manifests, formreaderfile.New(), resolver, candidates, publisher))

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
	clusterDirectory, err := clusterdirectoryhttp.New(baseURL, client)
	if err != nil {
		return err
	}
	connectCluster := clusterconnectcommandcli.New(
		usecases.NewConnectCluster(clusterAccess, clusterprovisionerk8s.New(), clusterRegistrar, clusterDirectory),
	)

	operatorSource, err := databaseoperatorhttp.New(baseURL, client)
	if err != nil {
		return err
	}
	installOperator := databaseoperatorinstallcommandcli.New(
		usecases.NewInstallOperator(operatorSource, clusterDirectory, operatorinstallerk8s.New()),
	)

	login := authlogincommandcli.New(usecases.NewBrowserLogin(browserauthorizerlocal.New(oauth, os.Stderr), sessions))
	// Проверка входа OIDC идёт клиентом платформы: тем же токеном доступа, с тем
	// же продлением.
	whoami := authwhoamicommandcli.New(usecases.NewWhoAmI(sessions, userinfohydra.New(oauthURL, client)))
	logout := authlogoutcommandcli.New(usecases.NewLogout(sessions, oauth))

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
				Name:  "databases",
				Usage: "СУБД организации",
				Commands: []*cli.Command{
					{
						Name:     "operators",
						Usage:    "операторы СУБД в подключённых кластерах",
						Commands: []*cli.Command{installOperator.CLICommand()},
					},
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
