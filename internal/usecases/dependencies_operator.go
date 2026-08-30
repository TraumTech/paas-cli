package usecases

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/entities"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_operator_mock_test.go -package=usecases github.com/TraumTech/paas-cli/internal/usecases DatabaseOperatorSource,ClusterDirectory,OperatorInstaller

// DatabaseOperatorSource — оператор СУБД от платформы: манифест и право,
// которое ей нужно после установки. Не зашито в CLI по той же причине, что и
// права подключения кластера: обновление оператора — дело платформы.
type DatabaseOperatorSource interface {
	Operator(ctx context.Context, engine string) (*entities.DatabaseOperator, error)
}

// ClusterDirectory — подключённые кластеры организации.
type ClusterDirectory interface {
	ListClusters(ctx context.Context) ([]entities.ConnectedCluster, error)
}

// OperatorInstaller применяет манифест оператора локальным доступом владельца
// и выдаёт учётной записи платформы право из оператора.
type OperatorInstaller interface {
	Target(kubeconfig, contextName string) (*ClusterTarget, error)
	// Objects разбирает манифест, ничего не применяя: план показывается до
	// подтверждения.
	Objects(manifest string) ([]entities.ManifestObject, error)
	// AccountName — какой учётной записи платформы достанется право.
	AccountName() string
	// Install идемпотентна: повтор приводит кластер к манифесту и отчитывается,
	// что изменилось; ждёт готовности оператора.
	Install(ctx context.Context, kubeconfig, contextName string, op *entities.DatabaseOperator) (*entities.OperatorInstallReport, error)
}
