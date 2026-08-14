package authlogincommandcli

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_mock_test.go -package=authlogincommandcli github.com/TraumTech/paas-cli/internal/controllers/auth_login_command_cli UserLogin,BrowserLogin

// UserLogin — use case входа пользователя паролем; интерфейс держим в
// контроллере для тестируемости команды.
type UserLogin interface {
	Execute(ctx context.Context, in usecases.LoginInput) (*usecases.LoginResult, error)
}

// BrowserLogin — use case входа через браузер (AUTH-22): подтверждение и выдача
// токена происходят в интерфейсе платформы, пароль в терминал не вводится.
type BrowserLogin interface {
	Execute(ctx context.Context) (*usecases.BrowserLoginResult, error)
}
