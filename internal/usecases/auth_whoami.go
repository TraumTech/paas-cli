package usecases

import (
	"context"
	"fmt"
)

// WhoAmIUseCase отвечает, под кем выполнен вход. Сохранённый вход не берётся на
// веру: он проверяется у сервера авторизации, поэтому истёкший или отозванный
// вход честно сообщается пользователю.
type WhoAmIUseCase struct {
	sessions SessionStore
	login    LoginInspector
}

func NewWhoAmI(sessions SessionStore, login LoginInspector) *WhoAmIUseCase {
	return &WhoAmIUseCase{sessions: sessions, login: login}
}

func (uc *WhoAmIUseCase) Execute(ctx context.Context) (*WhoAmIResult, error) {
	if _, err := uc.sessions.Load(ctx); err != nil {
		return nil, fmt.Errorf("load credential: %w", err)
	}
	user, err := uc.login.Inspect(ctx)
	if err != nil {
		return nil, fmt.Errorf("inspect login: %w", err)
	}
	return &WhoAmIResult{Email: user.Email, UserID: user.UserID}, nil
}
