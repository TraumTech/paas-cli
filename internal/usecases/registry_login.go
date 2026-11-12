package usecases

import (
	"context"
	"fmt"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type RegistryLoginSource interface {
	RegistryLogin(ctx context.Context) (*entities.RegistryLogin, error)
}

type DockerLogin interface {
	DockerLogin(ctx context.Context, server, username, password string) error
}

// RegistryLoginUseCase входит в реестр образов платформы от имени CI сервиса
// (TRA-499) и отдаёт адрес репозитория образов сервиса.
type RegistryLoginUseCase struct {
	source RegistryLoginSource
	docker DockerLogin
}

func NewRegistryLogin(s RegistryLoginSource, d DockerLogin) *RegistryLoginUseCase {
	return &RegistryLoginUseCase{source: s, docker: d}
}

func (uc *RegistryLoginUseCase) Execute(ctx context.Context) (*entities.RegistryLogin, error) {
	login, err := uc.source.RegistryLogin(ctx)
	if err != nil {
		return nil, err
	}
	if err := uc.docker.DockerLogin(ctx, login.Server, login.Username, login.Password); err != nil {
		return nil, fmt.Errorf("войти в реестр: %w", err)
	}
	return login, nil
}
