package usecases

import (
	"context"
	"fmt"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type CheckCompatibilityUseCase struct {
	reader CandidateReader
	source CompatibilitySource
}

func NewCheckCompatibility(reader CandidateReader, source CompatibilitySource) *CheckCompatibilityUseCase {
	return &CheckCompatibilityUseCase{reader: reader, source: source}
}

func (uc *CheckCompatibilityUseCase) Execute(ctx context.Context, in CheckCompatibilityInput) (*entities.CompatibilityReport, error) {
	document, err := uc.reader.Read(ctx, in.CandidatePath)
	if err != nil {
		return nil, fmt.Errorf("read candidate: %w", err)
	}
	candidate := &entities.CandidateContract{Format: in.Format, Document: document}
	if err := candidate.Validate(); err != nil {
		return nil, fmt.Errorf("validate candidate: %w", err)
	}
	report, err := uc.source.CheckCompatibility(ctx, in.ServiceID, in.Name, in.Format, candidate.Document)
	if err != nil {
		return nil, fmt.Errorf("check compatibility: %w", err)
	}
	return report, nil
}

// CheckManifestCompatibilityUseCase — досрочная проверка всех контрактов
// манифеста против их протоколов (CLI-23): манифестный режим той же команды,
// которой прежде передавали сервис и файл кандидата руками. Сначала — гейт
// исчезнувшего протокола (с потребителями — ошибка), затем каждая запись
// сверяется со снимками потребителей своего имени.
type CheckManifestCompatibilityUseCase struct {
	manifests ManifestReader
	resolver  ServiceResolver
	reader    CandidateReader
	source    CompatibilitySource
	registry  RegistryDirectory
}

func NewCheckManifestCompatibility(manifests ManifestReader, resolver ServiceResolver, reader CandidateReader, source CompatibilitySource, registry RegistryDirectory) *CheckManifestCompatibilityUseCase {
	return &CheckManifestCompatibilityUseCase{manifests: manifests, resolver: resolver, reader: reader, source: source, registry: registry}
}

func (uc *CheckManifestCompatibilityUseCase) Execute(ctx context.Context, in CheckManifestCompatibilityInput) (*entities.ManifestCompatibilityReport, error) {
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

	reports := make([]entities.NamedCompatibilityReport, 0, len(loaded.protocols))
	for i, p := range loaded.protocols {
		report, err := uc.source.CheckCompatibility(ctx, serviceID, p.Name, loaded.contracts[i].Format, loaded.contracts[i].Document)
		if err != nil {
			return nil, protocolError(p.Name, fmt.Errorf("check compatibility: %w", err))
		}
		reports = append(reports, entities.NamedCompatibilityReport{Name: p.Name, Report: *report})
	}
	return &entities.ManifestCompatibilityReport{Reports: reports, Orphaned: orphaned}, nil
}
