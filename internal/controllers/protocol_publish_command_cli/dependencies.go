package protocolpublishcommandcli

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_mock_test.go -package=protocolpublishcommandcli github.com/TraumTech/paas-cli/internal/controllers/protocol_publish_command_cli ProtocolPublisher

// ProtocolPublisher — use case публикации протокола под версией; интерфейс держим
// в контроллере для тестируемости команды.
type ProtocolPublisher interface {
	Execute(ctx context.Context, in usecases.PublishProtocolInput) (*entities.ProtocolPublication, error)
}
