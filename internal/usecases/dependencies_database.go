package usecases

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// DatabaseAccess — доступ владельца к СУБД CNPG в кластере его kubeconfig.
// Платформа здесь не участвует: это личный вход владельца, а не выдача
// доступа клиентам.
type DatabaseAccess interface {
	Connect(kubeconfig, contextName string) (DatabaseCluster, error)
}

type DatabaseCluster interface {
	ContextName() string
	Find(ctx context.Context, name, namespace string) (*entities.DatabaseServer, error)
	// EnsureRole заводит роль и её пароль и ждёт, пока оператор её применит.
	EnsureRole(ctx context.Context, server entities.DatabaseServer, role string) error
	Credentials(ctx context.Context, server entities.DatabaseServer, role string) (*entities.DatabaseCredentials, error)
	// Forward держит один сеанс проброса 127.0.0.1:port → основной инстанс и
	// возвращается, когда связь потеряна или отменён ctx. ready вызывается,
	// когда порт слушает.
	Forward(ctx context.Context, server entities.DatabaseServer, port int, ready func()) error
}
