package usecases

import (
	"context"
	"fmt"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type PublishVersionUseCase struct {
	manifests ManifestReader
	resolver  ServiceResolver
	publisher VersionPublisher
}

func NewPublishVersion(manifests ManifestReader, resolver ServiceResolver, p VersionPublisher) *PublishVersionUseCase {
	return &PublishVersionUseCase{manifests: manifests, resolver: resolver, publisher: p}
}

func (uc *PublishVersionUseCase) Execute(ctx context.Context, in PublishVersionInput) (*entities.Version, error) {
	request := entities.VersionRequest{CommitRevision: in.CommitRevision}
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("validate version request: %w", err)
	}
	manifest, err := uc.manifests.Read(ctx, in.ManifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	name, err := manifest.ServiceName()
	if err != nil {
		return nil, err
	}
	serviceID, err := resolveSelfID(ctx, uc.resolver, name)
	if err != nil {
		return nil, err
	}
	version, err := uc.publisher.PublishVersion(ctx, serviceID, request.CommitRevision)
	if err != nil {
		return nil, fmt.Errorf("publish version: %w", err)
	}
	return version, nil
}
