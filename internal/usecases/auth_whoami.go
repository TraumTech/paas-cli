package usecases

import (
	"context"
	"fmt"
)

// WhoAmIUseCase отвечает, под кем выполнен вход: сохранённый токен сессии
// проверяется у identity-провайдера, а не берётся на веру — истёкшая или
// отозванная сессия честно сообщается пользователю.
type WhoAmIUseCase struct {
	sessions  SessionStore
	inspector SessionInspector
}

func NewWhoAmI(sessions SessionStore, inspector SessionInspector) *WhoAmIUseCase {
	return &WhoAmIUseCase{sessions: sessions, inspector: inspector}
}

func (uc *WhoAmIUseCase) Execute(ctx context.Context) (*WhoAmIResult, error) {
	token, err := uc.sessions.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load session: %w", err)
	}
	session, err := uc.inspector.Inspect(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("inspect session: %w", err)
	}
	return &WhoAmIResult{Email: session.Email}, nil
}
