package usecases

import (
	"context"
	"fmt"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type RegisterDependencyUseCase struct {
	manifests ManifestReader
	resolver  ServiceResolver
	reader    CandidateReader
	registrar DependencyRegistrar
}

func NewRegisterDependency(manifests ManifestReader, resolver ServiceResolver, reader CandidateReader, registrar DependencyRegistrar) *RegisterDependencyUseCase {
	return &RegisterDependencyUseCase{manifests: manifests, resolver: resolver, reader: reader, registrar: registrar}
}

func (uc *RegisterDependencyUseCase) Execute(ctx context.Context, in RegisterDependencyInput) (*entities.Dependency, error) {
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
	document, err := uc.reader.Read(ctx, in.ContractPath)
	if err != nil {
		return nil, fmt.Errorf("read contract: %w", err)
	}
	contract := &entities.CandidateContract{Document: document}
	if err := contract.Validate(); err != nil {
		return nil, fmt.Errorf("validate contract: %w", err)
	}
	dependency, err := uc.registrar.RegisterDependency(ctx, serviceID, in.VersionID, in.ProducerServiceID, contract.Document)
	if err != nil {
		return nil, fmt.Errorf("register dependency: %w", err)
	}
	return dependency, nil
}
