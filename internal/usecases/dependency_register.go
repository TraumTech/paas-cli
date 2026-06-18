package usecases

import (
	"context"
	"fmt"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// RegisterDependencyUseCase регистрирует в реестре весь состав зависимостей версии
// потребителя из его манифеста: продьюсеры берутся из объявленных зависимостей, а
// снимки их контрактов — из раскладки потребителя (та же, что наполняет sync).
type RegisterDependencyUseCase struct {
	manifests ManifestReader
	resolver  ServiceResolver
	reader    CandidateReader
	registrar DependencyRegistrar
}

func NewRegisterDependency(manifests ManifestReader, resolver ServiceResolver, reader CandidateReader, registrar DependencyRegistrar) *RegisterDependencyUseCase {
	return &RegisterDependencyUseCase{manifests: manifests, resolver: resolver, reader: reader, registrar: registrar}
}

func (uc *RegisterDependencyUseCase) Execute(ctx context.Context, in RegisterDependencyInput) (*RegisterDependenciesResult, error) {
	manifest, err := uc.manifests.Read(ctx, in.ManifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("validate manifest: %w", err)
	}
	consumerName := manifest.Service.Name

	// Резолвим потребителя и всех продьюсеров одним запросом: манифест адресует по
	// имени, платформа — по id.
	names := make([]string, 0, len(manifest.Dependencies)+1)
	names = append(names, consumerName)
	for _, dep := range manifest.Dependencies {
		names = append(names, dep.Name)
	}
	ids, err := uc.resolver.ResolveIDs(ctx, names)
	if err != nil {
		return nil, fmt.Errorf("resolve services: %w", err)
	}
	consumerID, ok := ids[consumerName]
	if !ok {
		return nil, fmt.Errorf("сервис %q: %w", consumerName, entities.ErrServiceNotFound)
	}

	dest := manifest.EffectiveDestination()
	registered := make([]RegisteredDependency, 0, len(manifest.Dependencies))
	for _, dep := range manifest.Dependencies {
		// Любая упавшая зависимость валит весь прогон: состав регистрируется целиком
		// или никак, а не частично и молча. Имя зависимости — в ошибке.
		producerID, ok := ids[dep.Name]
		if !ok {
			return nil, fmt.Errorf("зависимость %q: %w", dep.Name, entities.ErrServiceNotFound)
		}
		document, err := uc.reader.Read(ctx, entities.ContractSnapshotPath(dest, dep.Name))
		if err != nil {
			return nil, fmt.Errorf("зависимость %q: read snapshot: %w", dep.Name, err)
		}
		contract := &entities.CandidateContract{Document: document}
		if err := contract.Validate(); err != nil {
			return nil, fmt.Errorf("зависимость %q: %w", dep.Name, err)
		}
		if _, err := uc.registrar.RegisterDependency(ctx, consumerID, in.VersionID, producerID, contract.Document); err != nil {
			return nil, fmt.Errorf("зависимость %q: %w", dep.Name, err)
		}
		registered = append(registered, RegisteredDependency{ProducerName: dep.Name, ProducerServiceID: producerID})
	}

	return &RegisterDependenciesResult{Registered: registered}, nil
}
