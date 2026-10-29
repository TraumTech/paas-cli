package authlogincommandcli

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_mock_test.go -package=authlogincommandcli github.com/TraumTech/paas-cli/internal/controllers/auth_login_command_cli BrowserLogin

// BrowserLogin — use case входа через браузер (IAP-03): человек входит на
// странице платформы, пароль в терминал не вводится.
type BrowserLogin interface {
	Execute(ctx context.Context) (*usecases.BrowserLoginResult, error)
}
