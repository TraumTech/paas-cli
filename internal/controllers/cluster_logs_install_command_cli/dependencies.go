package clusterlogsinstallcommandcli

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_mock_test.go -package=clusterlogsinstallcommandcli github.com/TraumTech/paas-cli/internal/controllers/cluster_logs_install_command_cli LogCollectorInstaller

type LogCollectorInstaller interface {
	Execute(ctx context.Context, input usecases.InstallLogCollectorInput, confirm usecases.InstallLogCollectorConfirmFunc) (*usecases.InstallLogCollectorResult, error)
}
