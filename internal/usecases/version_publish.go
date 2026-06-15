package usecases

import (
	"context"
	"fmt"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type PublishVersionUseCase struct {
	publisher VersionPublisher
}

func NewPublishVersion(p VersionPublisher) *PublishVersionUseCase {
	return &PublishVersionUseCase{publisher: p}
}

func (uc *PublishVersionUseCase) Execute(ctx context.Context, in PublishVersionInput) (*entities.Version, error) {
	request := entities.VersionRequest{CommitRevision: in.CommitRevision}
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("validate version request: %w", err)
	}
	version, err := uc.publisher.PublishVersion(ctx, in.ServiceID, request.CommitRevision)
	if err != nil {
		return nil, fmt.Errorf("publish version: %w", err)
	}
	return version, nil
}
