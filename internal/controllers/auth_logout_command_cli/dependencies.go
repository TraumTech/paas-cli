package authlogoutcommandcli

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_mock_test.go -package=authlogoutcommandcli github.com/TraumTech/paas-cli/internal/controllers/auth_logout_command_cli UserLogout

// UserLogout — use case выхода; интерфейс держим в контроллере для тестируемости
// команды.
type UserLogout interface {
	Execute(ctx context.Context) (*usecases.LogoutResult, error)
}
