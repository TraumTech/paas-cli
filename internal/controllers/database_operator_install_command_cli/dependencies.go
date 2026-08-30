package databaseoperatorinstallcommandcli

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_mock_test.go -package=databaseoperatorinstallcommandcli github.com/TraumTech/paas-cli/internal/controllers/database_operator_install_command_cli OperatorInstaller

type OperatorInstaller interface {
	Execute(ctx context.Context, input usecases.InstallOperatorInput, confirm usecases.InstallOperatorConfirmFunc) (*usecases.InstallOperatorResult, error)
}
