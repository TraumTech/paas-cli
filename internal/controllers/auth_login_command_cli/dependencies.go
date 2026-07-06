package authlogincommandcli

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_mock_test.go -package=authlogincommandcli github.com/TraumTech/paas-cli/internal/controllers/auth_login_command_cli UserLogin

// UserLogin — use case входа пользователя; интерфейс держим в контроллере для
// тестируемости команды.
type UserLogin interface {
	Execute(ctx context.Context, in usecases.LoginInput) (*usecases.LoginResult, error)
}
