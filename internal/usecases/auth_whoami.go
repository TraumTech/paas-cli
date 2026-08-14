package usecases

import (
	"context"
	"fmt"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// WhoAmIUseCase отвечает, под кем выполнен вход. Сохранённый вход не берётся на
// веру: сессия проверяется у identity-провайдера, личный токен — у платформы
// (она же владеет его сроком и отзывом), поэтому истёкший или отозванный вход
// честно сообщается пользователю.
type WhoAmIUseCase struct {
	sessions  SessionStore
	inspector SessionInspector
	tokens    PersonalTokenDirectory
}

func NewWhoAmI(sessions SessionStore, inspector SessionInspector, tokens PersonalTokenDirectory) *WhoAmIUseCase {
	return &WhoAmIUseCase{sessions: sessions, inspector: inspector, tokens: tokens}
}

func (uc *WhoAmIUseCase) Execute(ctx context.Context) (*WhoAmIResult, error) {
	credential, err := uc.sessions.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load credential: %w", err)
	}

	if credential.Kind == entities.CredentialPersonalToken {
		return uc.inspectPersonalToken(ctx, credential)
	}

	session, err := uc.inspector.Inspect(ctx, credential.Token)
	if err != nil {
		return nil, fmt.Errorf("inspect session: %w", err)
	}
	return &WhoAmIResult{Email: session.Email}, nil
}

// inspectPersonalToken подтверждает вход самим обращением к платформе: перечень
// личных токенов доступен только их владельцу, а отсутствие в нём предъявленного
// токена означает, что его отозвали из другого места.
func (uc *WhoAmIUseCase) inspectPersonalToken(ctx context.Context, credential *entities.Credential) (*WhoAmIResult, error) {
	tokens, err := uc.tokens.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list personal tokens: %w", err)
	}
	for _, token := range tokens {
		if token.ID == credential.TokenID {
			return &WhoAmIResult{
				Email:     credential.Email,
				TokenName: token.Name,
				ExpiresAt: token.ExpiresAt,
			}, nil
		}
	}
	return nil, entities.ErrPersonalTokenRejected
}
