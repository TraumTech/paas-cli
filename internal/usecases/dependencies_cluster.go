package usecases

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/entities"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_cluster_mock_test.go -package=usecases github.com/TraumTech/paas-cli/internal/usecases ClusterAccessSource,ClusterProvisioner,ClusterRegistrar

// ClusterAccessSource — что платформа просит в подключаемом кластере. Список
// приходит от неё, а не зашит в CLI: иначе он устареет с первым же изменением
// требований, а команда об этом не узнает.
type ClusterAccessSource interface {
	RequiredAccess(ctx context.Context) ([]entities.AccessRule, error)
}

// ClusterTarget — координаты кластера, взятые из локального доступа владельца.
type ClusterTarget struct {
	Endpoint      string
	CACertificate string
	// ContextName — какой контекст выбран; показывается владельцу.
	ContextName string
}

// ClusterProvisioner заводит в кластере учётную запись для платформы, пользуясь
// локальным доступом владельца. Личный креденшел наружу не отдаётся.
type ClusterProvisioner interface {
	// Target читает координаты кластера, ничего не меняя: нужен до
	// подтверждения, чтобы показать владельцу, куда именно команда пойдёт.
	Target(kubeconfig, contextName string) (*ClusterTarget, error)
	// AccountName — под каким именем заводится учётная запись.
	AccountName() string
	// Provision идемпотентна: повтор переиспользует уже созданное, а не
	// задваивает — иначе неудача на регистрации оставляла бы мусор.
	Provision(ctx context.Context, kubeconfig, contextName string, rules []entities.AccessRule) (*entities.ClusterCredential, error)
}

// ClusterRegistrar регистрирует подключение на платформе.
type ClusterRegistrar interface {
	Register(ctx context.Context, name string, credential entities.ClusterCredential) (*entities.ConnectedCluster, error)
}
