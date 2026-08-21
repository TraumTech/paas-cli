package usecases

import (
	"context"
	"fmt"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type FetchProtocolUseCase struct {
	source ProtocolSource
	store  ProtocolStore
}

func NewFetchProtocol(source ProtocolSource, store ProtocolStore) *FetchProtocolUseCase {
	return &FetchProtocolUseCase{source: source, store: store}
}

func (uc *FetchProtocolUseCase) Execute(ctx context.Context, in FetchProtocolInput) (*FetchProtocolResult, error) {
	// Сужение до методов и срез до атрибутов выполняет платформа (CLI-09,
	// PRT-29) — CLI получает уже частичный контракт.
	protocol, narrowingSkipped, attributeNarrowingSkipped, err := uc.source.FetchProtocol(ctx, in.ServiceID, in.Methods, in.Attributes)
	if err != nil {
		return nil, fmt.Errorf("fetch protocol: %w", err)
	}
	// Формат без движка сужения у явного fetch — ошибка: пользователь просил
	// срез, молча класть без него нельзя.
	if narrowingSkipped {
		return nil, entities.ErrMethodsUnsupportedForFormat
	}
	if attributeNarrowingSkipped {
		return nil, entities.ErrAttributesUnsupportedForFormat
	}
	if err := protocol.Validate(); err != nil {
		return nil, fmt.Errorf("validate protocol: %w", err)
	}
	path, err := uc.store.Save(ctx, protocol, in.Destination)
	if err != nil {
		return nil, fmt.Errorf("save protocol: %w", err)
	}
	return &FetchProtocolResult{
		ServiceName:   protocol.ServiceName,
		VersionNumber: protocol.VersionNumber,
		Format:        protocol.Format,
		Path:          path,
	}, nil
}
