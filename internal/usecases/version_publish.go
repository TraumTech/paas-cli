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
	if err := entities.ValidateEnvironment(in.Environment); err != nil {
		return nil, err
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
	declaration, err := uc.forms.Read(ctx, in.FormPath)
	if err != nil {
		return nil, fmt.Errorf("read form: %w", err)
	}
	var form *entities.VersionForm
	if declaration != nil {
		if in.Image == "" {
			return nil, entities.ErrFormRequiresImage
		}
		if err := declaration.Validate(); err != nil {
			return nil, err
		}
		// Значения окружений разрешаются здесь: версия принадлежит окружению,
		// и платформа получает готовый набор, а не правило слияния (DEP-14/15).
		form = declaration.Resolve(in.Environment)
	}
	version, err := uc.publisher.PublishVersion(ctx, serviceID, in.Environment, request.CommitRevision, in.Branch, in.Image, form)
	if err != nil {
		return nil, fmt.Errorf("publish version: %w", err)
	}
	return version, nil
}
