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
	snapshots CandidateReader
	publisher BuildPublisher
}

func NewPublishBuild(manifests ManifestReader, forms FormReader, resolver ServiceResolver, snapshots CandidateReader, p BuildPublisher) *PublishBuildUseCase {
	return &PublishBuildUseCase{manifests: manifests, forms: forms, resolver: resolver, snapshots: snapshots, publisher: p}
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
	if request.Dependencies, err = uc.dependencies(ctx, manifest); err != nil {
		return nil, err
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

// dependencies собирает [[dependencies]] манифеста со снимками из раскладки —
// тем же путём, что dependencies register, только регистрирует их не CLI, а
// выкатка, уже зная версию (DEP-25). Состав едет целиком или публикация падает.
func (uc *PublishBuildUseCase) dependencies(ctx context.Context, manifest *entities.Manifest) ([]entities.BuildDependency, error) {
	if len(manifest.Dependencies) == 0 {
		return nil, nil
	}
	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("validate manifest: %w", err)
	}
	names := make([]string, 0, len(manifest.Dependencies))
	for _, dep := range manifest.Dependencies {
		names = append(names, dep.Name)
	}
	ids, err := uc.resolver.ResolveIDs(ctx, names)
	if err != nil {
		return nil, fmt.Errorf("resolve services: %w", err)
	}

	dest := manifest.EffectiveDestination()
	out := make([]entities.BuildDependency, 0, len(manifest.Dependencies))
	for _, dep := range manifest.Dependencies {
		producerID, ok := ids[dep.Name]
		if !ok {
			return nil, fmt.Errorf("зависимость %q: %w", dep.Name, entities.ErrServiceNotFound)
		}
		document, format, err := readDependencySnapshot(ctx, uc.snapshots, dest, dep.Name)
		if err != nil {
			return nil, fmt.Errorf("зависимость %q: %w", dep.Name, err)
		}
		contract := &entities.CandidateContract{Format: format, Document: document}
		if err := contract.Validate(); err != nil {
			return nil, fmt.Errorf("зависимость %q: %w", dep.Name, err)
		}
		out = append(out, entities.BuildDependency{
			ProducerServiceID: producerID,
			Format:            format,
			Document:          contract.Document,
			Methods:           dep.Methods,
			Waived:            dep.Waived,
		})
	}
	return out, nil
}
