package usecases

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type PublishProtocolUseCase struct {
	manifests ManifestReader
	resolver  ServiceResolver
	reader    CandidateReader
	publisher ProtocolPublisher
}

func NewPublishProtocol(manifests ManifestReader, resolver ServiceResolver, reader CandidateReader, publisher ProtocolPublisher) *PublishProtocolUseCase {
	return &PublishProtocolUseCase{manifests: manifests, resolver: resolver, reader: reader, publisher: publisher}
}

func (uc *PublishProtocolUseCase) Execute(ctx context.Context, in PublishProtocolInput) (*entities.ProtocolPublication, error) {
	manifest, err := uc.manifests.Read(ctx, in.ManifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	service, err := manifest.RequireService()
	if err != nil {
		return nil, err
	}

	ids, err := uc.resolver.ResolveIDs(ctx, []string{service.Name})
	if err != nil {
		return nil, fmt.Errorf("resolve service: %w", err)
	}
	serviceID, ok := ids[service.Name]
	if !ok {
		return nil, fmt.Errorf("сервис %q: %w", service.Name, entities.ErrServiceNotFound)
	}

	document, err := uc.reader.Read(ctx, contractPath(in.ManifestPath, service.Contract))
	if err != nil {
		return nil, fmt.Errorf("read contract: %w", err)
	}
	contract := &entities.CandidateContract{Document: document}
	if err := contract.Validate(); err != nil {
		return nil, fmt.Errorf("validate contract: %w", err)
	}
	publication, err := uc.publisher.PublishProtocol(ctx, serviceID, in.VersionID, contract.Document)
	if err != nil {
		return nil, fmt.Errorf("publish protocol: %w", err)
	}
	return publication, nil
}

// contractPath разрешает путь к контракту относительно самого манифеста: контракт
// лежит рядом с ним в репозитории, поэтому команда работает из любого каталога.
func contractPath(manifestPath, contract string) string {
	if filepath.IsAbs(contract) {
		return contract
	}
	return filepath.Join(filepath.Dir(manifestPath), contract)
}
