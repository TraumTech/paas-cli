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
	// Сужение до методов выполняет платформа (CLI-09) — CLI получает уже
	// частичный контракт.
	protocol, narrowingSkipped, err := uc.source.FetchProtocol(ctx, in.ServiceID, in.Methods)
	if err != nil {
		return nil, fmt.Errorf("fetch protocol: %w", err)
	}
	// Формат без поддержки сужения (gRPC) у явного fetch -m — ошибка, как и
	// раньше: пользователь просил срез, молча класть целиком нельзя.
	if narrowingSkipped {
		return nil, entities.ErrMethodsUnsupportedForGRPC
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
