package usecases

import (
	"context"
	"fmt"
)

type FetchProtocolUseCase struct {
	source ProtocolSource
	store  ProtocolStore
}

func NewFetchProtocol(source ProtocolSource, store ProtocolStore) *FetchProtocolUseCase {
	return &FetchProtocolUseCase{source: source, store: store}
}

func (uc *FetchProtocolUseCase) Execute(ctx context.Context, in FetchProtocolInput) (*FetchProtocolResult, error) {
	protocol, err := uc.source.FetchProtocol(ctx, in.ServiceID)
	if err != nil {
		return nil, fmt.Errorf("fetch protocol: %w", err)
	}
	if err := protocol.Validate(); err != nil {
		return nil, fmt.Errorf("validate protocol: %w", err)
	}
	if len(in.Methods) > 0 {
		protocol, err = protocol.SelectMethods(in.Methods)
		if err != nil {
			return nil, fmt.Errorf("select methods: %w", err)
		}
	}
	path, err := uc.store.Save(ctx, protocol, in.Destination)
	if err != nil {
		return nil, fmt.Errorf("save protocol: %w", err)
	}
	return &FetchProtocolResult{
		ServiceName:   protocol.ServiceName,
		VersionNumber: protocol.VersionNumber,
		Path:          path,
	}, nil
}
