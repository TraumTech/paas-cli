package usecases

import (
	"context"
	"fmt"
	"os"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type PublishBuildUseCase struct {
	manifests ManifestReader
	forms     FormReader
	resolver  ServiceResolver
	publisher BuildPublisher
}

func NewPublishBuild(manifests ManifestReader, forms FormReader, resolver ServiceResolver, p BuildPublisher) *PublishBuildUseCase {
	return &PublishBuildUseCase{manifests: manifests, forms: forms, resolver: resolver, publisher: p}
}

// Execute публикует сборку ветки (DEP-18). В отличие от публикации версии,
// окружение не называется и секции [env.*] не разрешаются: их сливает выкатка,
// когда окружение выбрано. Контракт едет тем же артефактом — публикует его в
// реестр тоже выкатка.
func (uc *PublishBuildUseCase) Execute(ctx context.Context, in PublishBuildInput) (*entities.Build, error) {
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

	declaration, err := uc.forms.Read(ctx, in.FormPath)
	if err != nil {
		return nil, fmt.Errorf("read form: %w", err)
	}
	request := entities.BuildRequest{
		CommitRevision: in.CommitRevision,
		Branch:         in.Branch,
		Image:          in.Image,
		Form:           declaration,
	}
	if declaration != nil && in.Image == "" {
		return nil, entities.ErrFormRequiresImage
	}
	// Контракт читаем из репозитория, но публикует его в реестр не CLI, а
	// выкатка — уже зная версию, к которой его привязать (DEP-19). Сборка несёт
	// ровно один контракт без имени (протокол по умолчанию): перечень
	// [[protocols]] из нескольких или именованных записей сборка пока не везёт.
	protocols, err := manifest.DeclaredProtocols()
	if err != nil {
		return nil, err
	}
	switch {
	case len(protocols) > 1:
		return nil, entities.ErrBuildMultipleContracts
	case len(protocols) == 1 && protocols[0].Name != "" && protocols[0].Name != entities.DefaultProtocolName:
		return nil, entities.ErrBuildNamedContract
	case len(protocols) == 1:
		document, err := os.ReadFile(protocols[0].Contract)
		if err != nil {
			return nil, fmt.Errorf("read contract %s: %w", protocols[0].Contract, err)
		}
		request.Contract = string(document)
		request.ContractFormat = protocols[0].Format
		if request.ContractFormat == "" {
			// Пустой формат в манифесте означает OpenAPI (ParseProtocolFormat).
			request.ContractFormat = "openapi"
		}
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}

	build, err := uc.publisher.PublishBuild(ctx, serviceID, request)
	if err != nil {
		return nil, fmt.Errorf("publish build: %w", err)
	}
	return build, nil
}
