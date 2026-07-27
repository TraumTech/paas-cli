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
		document, format, err := uc.readSnapshot(ctx, dest, dep.Name)
		if err != nil {
			return nil, fmt.Errorf("зависимость %q: %w", dep.Name, err)
		}
		contract := &entities.CandidateContract{Format: format, Document: document}
		if err := contract.Validate(); err != nil {
			return nil, fmt.Errorf("зависимость %q: %w", dep.Name, err)
		}
		if _, err := uc.registrar.RegisterDependency(ctx, DependencyRegistration{
			ServiceID:         consumerID,
			VersionID:         in.VersionID,
			ProducerServiceID: producerID,
			Format:            format,
			Document:          contract.Document,
			Methods:           dep.Methods,
			Waived:            dep.Waived,
			SupersedePrevious: in.SupersedePrevious,
		}); err != nil {
			return nil, fmt.Errorf("зависимость %q: %w", dep.Name, err)
		}
		registered = append(registered, RegisteredDependency{ProducerName: dep.Name, ProducerServiceID: producerID})
	}

	return &RegisterDependenciesResult{Registered: registered}, nil
}

// readSnapshot читает снимок зависимости из раскладки, определяя формат по имени
// файла, которым его положил sync (CLI-19): openapi.json — прежний формат,
// contract.proto — gRPC. Оба сразу — неоднозначность, а не молчаливый выбор.
func (uc *RegisterDependencyUseCase) readSnapshot(ctx context.Context, dest, name string) ([]byte, entities.ProtocolFormat, error) {
	openapiDoc, openapiErr := uc.reader.Read(ctx, entities.ContractSnapshotPath(dest, name, entities.ProtocolFormatOpenAPI))
	grpcDoc, grpcErr := uc.reader.Read(ctx, entities.ContractSnapshotPath(dest, name, entities.ProtocolFormatGRPC))
	switch {
	case openapiErr == nil && grpcErr == nil:
		return nil, "", fmt.Errorf("в раскладке два снимка (%s и %s) — оставьте один и повторите",
			entities.ProtocolFileName, entities.GRPCProtocolFileName)
	case openapiErr == nil:
		return openapiDoc, entities.ProtocolFormatOpenAPI, nil
	case grpcErr == nil:
		return grpcDoc, entities.ProtocolFormatGRPC, nil
	default:
		// Ни одного снимка: показываем ошибку по прежнему формату (привычный путь
		// в тексте), полная диагностика — выполнить protocols sync.
		return nil, "", fmt.Errorf("read snapshot: %w (нет и %s; выполните protocols sync)", openapiErr, entities.GRPCProtocolFileName)
	}
}
