package protocolsynccommandcli

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/usecases"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_mock_test.go -package=protocolsynccommandcli github.com/TraumTech/paas-cli/internal/controllers/protocol_sync_command_cli ProtocolSyncer

// ProtocolSyncer — use case синхронизации контрактов по манифесту; интерфейс держим
// в контроллере для тестируемости команды.
type ProtocolSyncer interface {
	Execute(ctx context.Context, in usecases.SyncProtocolsInput) (*usecases.SyncProtocolsResult, error)
}
