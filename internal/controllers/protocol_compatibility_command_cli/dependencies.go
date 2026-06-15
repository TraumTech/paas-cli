package protocolcompatibilitycommandcli

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_mock_test.go -package=protocolcompatibilitycommandcli github.com/TraumTech/paas-cli/internal/controllers/protocol_compatibility_command_cli CompatibilityChecker

// CompatibilityChecker — use case проверки совместимости кандидата; интерфейс
// держим в контроллере для тестируемости команды.
type CompatibilityChecker interface {
	Execute(ctx context.Context, in usecases.CheckCompatibilityInput) (*entities.CompatibilityReport, error)
}
