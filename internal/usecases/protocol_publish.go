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
	registry  RegistryDirectory
}

func NewPublishProtocol(manifests ManifestReader, resolver ServiceResolver, reader CandidateReader, publisher ProtocolPublisher, registry RegistryDirectory) *PublishProtocolUseCase {
	return &PublishProtocolUseCase{manifests: manifests, resolver: resolver, reader: reader, publisher: publisher, registry: registry}
}

// Execute публикует под версией все контракты из манифеста (CLI-23): каждую
// запись [[protocols]] под её именем, прежнюю форму contract в [service] — как
// протокол по умолчанию. До публикации — гейт исчезнувшего протокола: реестр
// сверяется с перечнем, осиротевший протокол с потребителями останавливает
// публикацию целиком (для них это ломающее изменение, как PRT-09), без
// потребителей — попадает в предупреждения отчёта.
func (uc *PublishProtocolUseCase) Execute(ctx context.Context, in PublishProtocolInput) (*entities.ProtocolPublishReport, error) {
	loaded, err := loadManifestContracts(ctx, uc.manifests, uc.reader, in.ManifestPath)
	if err != nil {
		return nil, err
	}
	serviceID, err := resolveSelfID(ctx, uc.resolver, loaded.manifest.Service.Name)
	if err != nil {
		return nil, err
	}
	orphaned, err := checkOrphanedProtocols(ctx, uc.registry, serviceID, loaded.protocols)
	if err != nil {
		return nil, err
	}

	publications := make([]entities.ProtocolPublication, 0, len(loaded.protocols))
	for i, p := range loaded.protocols {
		publication, err := uc.publisher.PublishProtocol(ctx, serviceID, in.VersionID, p.Name, loaded.contracts[i].Format, loaded.contracts[i].Document)
		if err != nil {
			return nil, protocolError(p.Name, fmt.Errorf("publish protocol: %w", err))
		}
		publication.Name = p.Name
		publications = append(publications, *publication)
	}
	return &entities.ProtocolPublishReport{Publications: publications, Orphaned: orphaned}, nil
}

// contractPath разрешает путь к контракту относительно самого манифеста: контракт
// лежит рядом с ним в репозитории, поэтому команда работает из любого каталога.
func contractPath(manifestPath, contract string) string {
	if filepath.IsAbs(contract) {
		return contract
	}
	return filepath.Join(filepath.Dir(manifestPath), contract)
}
