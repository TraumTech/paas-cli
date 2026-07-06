package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type LogoutUseCase struct {
	sessions SessionStore
	revoker  SessionRevoker
}

func NewLogout(sessions SessionStore, revoker SessionRevoker) *LogoutUseCase {
	return &LogoutUseCase{sessions: sessions, revoker: revoker}
}

func (uc *LogoutUseCase) Execute(ctx context.Context) (*LogoutResult, error) {
	token, err := uc.sessions.Load(ctx)
	if errors.Is(err, entities.ErrNoSession) {
		return &LogoutResult{WasLoggedIn: false}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}
	// Сначала завершаем сессию у провайдера, потом чистим локально: если отзыв
	// не прошёл (сеть), вход остаётся на месте и выход можно повторить.
	if err := uc.revoker.Revoke(ctx, token); err != nil {
		return nil, fmt.Errorf("revoke session: %w", err)
	}
	if err := uc.sessions.Delete(ctx); err != nil {
		return nil, fmt.Errorf("delete session: %w", err)
	}
	return &LogoutResult{WasLoggedIn: true}, nil
}
