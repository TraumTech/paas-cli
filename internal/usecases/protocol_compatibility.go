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
	candidate := &entities.CandidateContract{Document: document}
	if err := candidate.Validate(); err != nil {
		return nil, fmt.Errorf("validate candidate: %w", err)
	}
	report, err := uc.source.CheckCompatibility(ctx, in.ServiceID, candidate.Document)
	if err != nil {
		return nil, fmt.Errorf("check compatibility: %w", err)
	}
	return report, nil
}
