package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// LogoutUseCase завершает вход: сессию — у identity-провайдера, личный токен —
// на платформе (выданный браузером токен не должен пережить выход, AUTH-22).
type LogoutUseCase struct {
	sessions SessionStore
	revoker  SessionRevoker
	tokens   PersonalTokenDirectory
}

func NewLogout(sessions SessionStore, revoker SessionRevoker, tokens PersonalTokenDirectory) *LogoutUseCase {
	return &LogoutUseCase{sessions: sessions, revoker: revoker, tokens: tokens}
}

func (uc *LogoutUseCase) Execute(ctx context.Context) (*LogoutResult, error) {
	credential, err := uc.sessions.Load(ctx)
	if errors.Is(err, entities.ErrNoSession) {
		return &LogoutResult{WasLoggedIn: false}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load credential: %w", err)
	}
	// Сначала гасим вход на стороне платформы/провайдера, потом чистим локально:
	// если отзыв не прошёл (сеть), вход остаётся на месте и выход можно повторить.
	if err := uc.revokeCredential(ctx, credential); err != nil {
		return nil, err
	}
	if err := uc.sessions.Delete(ctx); err != nil {
		return nil, fmt.Errorf("delete credential: %w", err)
	}
	return &LogoutResult{WasLoggedIn: true}, nil
}

func (uc *LogoutUseCase) revokeCredential(ctx context.Context, credential *entities.Credential) error {
	if credential.Kind != entities.CredentialPersonalToken {
		if err := uc.revoker.Revoke(ctx, credential.Token); err != nil {
			return fmt.Errorf("revoke session: %w", err)
		}
		return nil
	}
	// Личный токен, выданный не браузерным входом (положен руками в файл), id не
	// несёт — отзывать нечего, достаточно забыть его локально.
	if credential.TokenID == "" {
		return nil
	}
	// Токен, отозванный из интерфейса раньше, — цель уже достигнута.
	if err := uc.tokens.Revoke(ctx, credential.TokenID); err != nil &&
		!errors.Is(err, entities.ErrPersonalTokenRejected) {
		return fmt.Errorf("revoke personal token: %w", err)
	}
	return nil
}
