package dependencyregistercommandcli

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_mock_test.go -package=dependencyregistercommandcli github.com/TraumTech/paas-cli/internal/controllers/dependency_register_command_cli DependencyRegistrar

// DependencyRegistrar — use case регистрации зависимости; интерфейс держим в
// контроллере для тестируемости команды.
type DependencyRegistrar interface {
	Execute(ctx context.Context, in usecases.RegisterDependencyInput) (*entities.Dependency, error)
}
