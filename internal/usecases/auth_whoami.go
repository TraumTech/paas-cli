package usecases

import (
	"context"
	"fmt"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// WhoAmIUseCase отвечает, под кем выполнен вход. Сохранённый вход не берётся на
// веру: вход OIDC проверяется у сервера авторизации, сессия — у
// identity-провайдера, личный токен — у платформы (она же владеет его сроком и
// отзывом), поэтому истёкший или отозванный вход честно сообщается пользователю.
type WhoAmIUseCase struct {
	sessions  SessionStore
	login     LoginInspector
	inspector SessionInspector
	tokens    PersonalTokenDirectory
}

func NewWhoAmI(sessions SessionStore, login LoginInspector, inspector SessionInspector, tokens PersonalTokenDirectory) *WhoAmIUseCase {
	return &WhoAmIUseCase{sessions: sessions, login: login, inspector: inspector, tokens: tokens}
}

func (uc *WhoAmIUseCase) Execute(ctx context.Context) (*WhoAmIResult, error) {
	credential, err := uc.sessions.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load credential: %w", err)
	}

	switch credential.Kind {
	case entities.CredentialOIDC:
		user, err := uc.login.Inspect(ctx)
		if err != nil {
			return nil, fmt.Errorf("inspect login: %w", err)
		}
		return &WhoAmIResult{Email: user.Email, UserID: user.UserID}, nil
	case entities.CredentialPersonalToken:
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
