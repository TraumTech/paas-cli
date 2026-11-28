package checkcommandcli

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

// ServiceFileChecker — use case проверки файла сервиса; интерфейс держим в
// контроллере для тестируемости команды.
type ServiceFileChecker interface {
	Execute(ctx context.Context, in usecases.CheckServiceFileInput) (*usecases.CheckServiceFileResult, error)
}
