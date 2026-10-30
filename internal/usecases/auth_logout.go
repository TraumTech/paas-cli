package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// LogoutUseCase завершает вход там, где он живёт: у сервера авторизации (отзыв
// refresh-токена), затем локально.
type LogoutUseCase struct {
	sessions SessionStore
	login    LoginRevoker
}

func NewLogout(sessions SessionStore, login LoginRevoker) *LogoutUseCase {
	return &LogoutUseCase{sessions: sessions, login: login}
}

func (uc *LogoutUseCase) Execute(ctx context.Context) (*LogoutResult, error) {
	credential, err := uc.sessions.Load(ctx)
	if errors.Is(err, entities.ErrNoSession) {
		// Файл входа прежнего вида тоже читается как «нет входа» — убираем его,
		// чтобы выход оставлял чистое место.
		if err := uc.sessions.Delete(ctx); err != nil {
			return nil, fmt.Errorf("delete credential: %w", err)
		}
		return &LogoutResult{WasLoggedIn: false}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load credential: %w", err)
	}
	// Сначала гасим вход у сервера авторизации, потом чистим локально: если
	// отзыв не прошёл (сеть), вход остаётся на месте и выход можно повторить.
	if err := uc.login.Revoke(ctx, credential.RefreshToken); err != nil {
		return nil, fmt.Errorf("revoke login: %w", err)
	}
	if err := uc.sessions.Delete(ctx); err != nil {
		return nil, fmt.Errorf("delete credential: %w", err)
	}
	return &LogoutResult{WasLoggedIn: true}, nil
}
