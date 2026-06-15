package versionpublishcommandcli

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_mock_test.go -package=versionpublishcommandcli github.com/TraumTech/paas-cli/internal/controllers/version_publish_command_cli VersionPublisher

// VersionPublisher — use case публикации версии; интерфейс держим в контроллере
// для тестируемости команды.
type VersionPublisher interface {
	Execute(ctx context.Context, in usecases.PublishVersionInput) (*entities.Version, error)
}
