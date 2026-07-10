package usecases

import (
	"context"
	"fmt"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// SyncProtocolsUseCase читает манифест зависимостей и тянет контракты всех
// объявленных сервисов: обобщение получения одного контракта на весь манифест.
type SyncProtocolsUseCase struct {
	manifests ManifestReader
	resolver  ServiceResolver
	source    ProtocolSource
	store     ProtocolStore
}

func NewSyncProtocols(manifests ManifestReader, resolver ServiceResolver, source ProtocolSource, store ProtocolStore) *SyncProtocolsUseCase {
	return &SyncProtocolsUseCase{manifests: manifests, resolver: resolver, source: source, store: store}
}

func (uc *SyncProtocolsUseCase) Execute(ctx context.Context, in SyncProtocolsInput) (*SyncProtocolsResult, error) {
	manifest, err := uc.manifests.Read(ctx, in.ManifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("validate manifest: %w", err)
	}

	dest := manifest.EffectiveDestination()
	if in.DestinationOverride != "" {
		dest = in.DestinationOverride
	}

	names := make([]string, 0, len(manifest.Dependencies))
	for _, dep := range manifest.Dependencies {
		names = append(names, dep.Name)
	}
	ids, err := uc.resolver.ResolveIDs(ctx, names)
	if err != nil {
		return nil, fmt.Errorf("resolve services: %w", err)
	}

	results := make([]FetchProtocolResult, 0, len(manifest.Dependencies))
	for _, dep := range manifest.Dependencies {
		// Любая упавшая зависимость валит весь прогон: пайплайн должен
		// остановиться, а не получить часть контрактов молча. Имя сервиса в ошибке
		// — чтобы было видно, на какой зависимости упало.
		serviceID, ok := ids[dep.Name]
		if !ok {
			return nil, fmt.Errorf("зависимость %q: %w", dep.Name, entities.ErrServiceNotFound)
		}
		// Сужение до методов выполняет платформа (CLI-09). methods у
		// gRPC-зависимости объявляют используемые методы для регистрации (CLI-20),
		// а сужение контракта для gRPC не поддерживается — платформа приносит его
		// целиком с narrowingSkipped, что честно отражаем в отчёте (CLI-19).
		protocol, narrowingSkipped, err := uc.source.FetchProtocol(ctx, serviceID, dep.Methods)
		if err != nil {
			return nil, fmt.Errorf("зависимость %q: %w", dep.Name, err)
		}
		if err := protocol.Validate(); err != nil {
			return nil, fmt.Errorf("зависимость %q: %w", dep.Name, err)
		}
		path, err := uc.store.Save(ctx, protocol, dest)
		if err != nil {
			return nil, fmt.Errorf("зависимость %q: %w", dep.Name, err)
		}
		results = append(results, FetchProtocolResult{
			ServiceName:      protocol.ServiceName,
			VersionNumber:    protocol.VersionNumber,
			Format:           protocol.Format,
			Path:             path,
			NarrowingSkipped: narrowingSkipped,
		})
	}

	return &SyncProtocolsResult{Destination: dest, Protocols: results}, nil
}
