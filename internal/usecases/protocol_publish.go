package usecases

import (
	"context"
	"fmt"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type PublishProtocolUseCase struct {
	reader    CandidateReader
	publisher ProtocolPublisher
}

func NewPublishProtocol(reader CandidateReader, publisher ProtocolPublisher) *PublishProtocolUseCase {
	return &PublishProtocolUseCase{reader: reader, publisher: publisher}
}

func (uc *PublishProtocolUseCase) Execute(ctx context.Context, in PublishProtocolInput) (*entities.ProtocolPublication, error) {
	document, err := uc.reader.Read(ctx, in.ContractPath)
	if err != nil {
		return nil, fmt.Errorf("read contract: %w", err)
	}
	contract := &entities.CandidateContract{Document: document}
	if err := contract.Validate(); err != nil {
		return nil, fmt.Errorf("validate contract: %w", err)
	}
	publication, err := uc.publisher.PublishProtocol(ctx, in.ServiceID, in.VersionID, contract.Document)
	if err != nil {
		return nil, fmt.Errorf("publish protocol: %w", err)
	}
	return publication, nil
}
