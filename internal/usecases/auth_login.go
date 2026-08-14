package usecases

import (
	"context"
	"fmt"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type LoginUseCase struct {
	authenticator CredentialAuthenticator
	sessions      SessionStore
}

func NewLogin(authenticator CredentialAuthenticator, sessions SessionStore) *LoginUseCase {
	return &LoginUseCase{authenticator: authenticator, sessions: sessions}
}

func (uc *LoginUseCase) Execute(ctx context.Context, in LoginInput) (*LoginResult, error) {
	if in.Email == "" || in.Password == "" {
		return nil, entities.ErrEmptyCredentials
	}
	session, err := uc.authenticator.Authenticate(ctx, in.Email, in.Password)
	if err != nil {
		// Неудачный вход ничего не сохраняет — прежний вход (если был) не затирается.
		return nil, fmt.Errorf("authenticate: %w", err)
	}
	if err := uc.sessions.Save(ctx, entities.Credential{
		Kind:  entities.CredentialSession,
		Token: session.Token,
		Email: session.Email,
	}); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}
	return &LoginResult{Email: session.Email}, nil
}
