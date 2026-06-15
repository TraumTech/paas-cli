package usecases

import (
	"context"
	"fmt"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type RegisterDependencyUseCase struct {
	reader    CandidateReader
	registrar DependencyRegistrar
}

func NewRegisterDependency(reader CandidateReader, registrar DependencyRegistrar) *RegisterDependencyUseCase {
	return &RegisterDependencyUseCase{reader: reader, registrar: registrar}
}

func (uc *RegisterDependencyUseCase) Execute(ctx context.Context, in RegisterDependencyInput) (*entities.Dependency, error) {
	document, err := uc.reader.Read(ctx, in.ContractPath)
	if err != nil {
		return nil, fmt.Errorf("read contract: %w", err)
	}
	contract := &entities.CandidateContract{Document: document}
	if err := contract.Validate(); err != nil {
		return nil, fmt.Errorf("validate contract: %w", err)
	}
	dependency, err := uc.registrar.RegisterDependency(ctx, in.ServiceID, in.VersionID, in.ProducerServiceID, contract.Document)
	if err != nil {
		return nil, fmt.Errorf("register dependency: %w", err)
	}
	return dependency, nil
}
