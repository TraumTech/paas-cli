package protocolfetchcommandcli

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_mock_test.go -package=protocolfetchcommandcli github.com/TraumTech/paas-cli/internal/controllers/protocol_fetch_command_cli ProtocolFetcher

// ProtocolFetcher — use case получения контракта; интерфейс держим в контроллере
// для тестируемости команды.
type ProtocolFetcher interface {
	Execute(ctx context.Context, in usecases.FetchProtocolInput) (*usecases.FetchProtocolResult, error)
}
