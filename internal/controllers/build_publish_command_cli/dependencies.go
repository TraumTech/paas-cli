package buildpublishcommandcli

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_mock_test.go -package=buildpublishcommandcli github.com/TraumTech/paas-cli/internal/controllers/build_publish_command_cli BuildPublisher

// BuildPublisher — зависимость команды: use case публикации сборки.
type BuildPublisher interface {
	Execute(ctx context.Context, in usecases.PublishBuildInput) (*entities.Build, error)
}
