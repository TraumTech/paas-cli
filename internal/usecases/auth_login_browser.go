package usecases

import (
	"context"
	"fmt"
)

// BrowserLoginUseCase — вход через браузер (AUTH-22): платформа выпускает личный
// токен по явному подтверждению человека, CLI получает его сам и сохраняет.
// Пароль при этом через терминал не проходит.
type BrowserLoginUseCase struct {
	authorizer BrowserAuthorizer
	sessions   SessionStore
}

func NewBrowserLogin(authorizer BrowserAuthorizer, sessions SessionStore) *BrowserLoginUseCase {
	return &BrowserLoginUseCase{authorizer: authorizer, sessions: sessions}
}

func (uc *BrowserLoginUseCase) Execute(ctx context.Context) (*BrowserLoginResult, error) {
	credential, err := uc.authorizer.Authorize(ctx)
	if err != nil {
		// Неподтверждённый вход ничего не сохраняет — прежний вход не затирается.
		return nil, fmt.Errorf("authorize in browser: %w", err)
	}
	if err := uc.sessions.Save(ctx, *credential); err != nil {
		return nil, fmt.Errorf("save credential: %w", err)
	}
	return &BrowserLoginResult{Email: credential.Email, ExpiresAt: credential.ExpiresAt}, nil
}
