package authwhoamicommandcli

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_mock_test.go -package=authwhoamicommandcli github.com/TraumTech/paas-cli/internal/controllers/auth_whoami_command_cli CurrentUser

// CurrentUser — use case «под кем выполнен вход»; интерфейс держим в контроллере
// для тестируемости команды.
type CurrentUser interface {
	Execute(ctx context.Context) (*usecases.WhoAmIResult, error)
}
