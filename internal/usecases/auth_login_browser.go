package usecases

import (
	"context"
	"fmt"
)

// BrowserLoginUseCase — вход через браузер (IAP-03): человек входит на странице
// платформы, сервер авторизации выдаёт CLI пару «токен доступа + refresh», CLI
// сохраняет её. Пароль при этом через терминал не проходит.
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
		// Неудавшийся вход ничего не сохраняет — прежний вход не затирается.
		return nil, fmt.Errorf("authorize in browser: %w", err)
	}
	if err := uc.sessions.Save(ctx, *credential); err != nil {
		return nil, fmt.Errorf("save credential: %w", err)
	}
	return &BrowserLoginResult{Email: credential.Email}, nil
}
