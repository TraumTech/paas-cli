package usecases

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/entities"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_log_collector_mock_test.go -package=usecases github.com/TraumTech/paas-cli/internal/usecases LogCollectorInstaller

// LogCollectorInstaller ставит сборщик вывода сервисов локальным доступом
// владельца кластера.
type LogCollectorInstaller interface {
	Target(kubeconfig, contextName string) (*ClusterTarget, error)
	// Objects — что появится в кластере; показывается до подтверждения.
	Objects(collector entities.LogCollector) ([]entities.ManifestObject, error)
	// Install идемпотентна: повтор приводит кластер к манифесту и ждёт, пока
	// сборщик поднимется на всех нодах.
	Install(ctx context.Context, kubeconfig, contextName string, collector entities.LogCollector) (*entities.OperatorInstallReport, error)
}
