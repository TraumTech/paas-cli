package usecases

import (
	"context"
	"fmt"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type PublishVersionUseCase struct {
	manifests ManifestReader
	forms     FormReader
	resolver  ServiceResolver
	publisher VersionPublisher
}

func NewPublishVersion(manifests ManifestReader, forms FormReader, resolver ServiceResolver, p VersionPublisher) *PublishVersionUseCase {
	return &PublishVersionUseCase{manifests: manifests, forms: forms, resolver: resolver, publisher: p}
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
	// Форма едет с версией (DEP-02): отсутствие paas.toml — штатно, публикация
	// без формы. Форма без образа не имеет смысла — отказываем до сети.
	form, err := uc.forms.Read(ctx, in.FormPath)
	if err != nil {
		return nil, fmt.Errorf("read form: %w", err)
	}
	if form != nil && in.Image == "" {
		return nil, entities.ErrFormRequiresImage
	}
	version, err := uc.publisher.PublishVersion(ctx, serviceID, request.CommitRevision, in.Image, form)
	if err != nil {
		return nil, fmt.Errorf("publish version: %w", err)
	}
	return version, nil
}
