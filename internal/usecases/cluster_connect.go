package usecases

import (
	"context"
	"fmt"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// ConnectClusterInput — что владелец указал в команде.
type ConnectClusterInput struct {
	// Name — под каким именем кластер появится на платформе.
	Name string
	// Context — контекст kubeconfig; пусто означает текущий.
	Context string
	// Kubeconfig — путь к файлу; пусто означает обычное разрешение (KUBECONFIG,
	// затем ~/.kube/config).
	Kubeconfig string
}

// ConnectClusterPlan — что команда собирается сделать в кластере владельца.
// Показывается до применения: это чужой кластер, и молча менять его нельзя.
type ConnectClusterPlan struct {
	// Endpoint — адрес кластера, к которому команда обратится.
	Endpoint string
	// ServiceAccount — имя учётной записи, которую заведёт команда.
	ServiceAccount string
	Rules          []entities.AccessRule
}

// ConfirmFunc спрашивает у владельца согласие на изменение кластера. Возврат
// false означает отказ — ничего не применяется.
type ConfirmFunc func(plan ConnectClusterPlan) (bool, error)

type ConnectClusterUseCase struct {
	access      ClusterAccessSource
	provisioner ClusterProvisioner
	registrar   ClusterRegistrar
}

func NewConnectCluster(a ClusterAccessSource, p ClusterProvisioner, r ClusterRegistrar) *ConnectClusterUseCase {
	return &ConnectClusterUseCase{access: a, provisioner: p, registrar: r}
}

// Execute подключает кластер: спрашивает у платформы, какие права ей нужны,
// показывает владельцу что будет создано, заводит учётную запись его же
// доступом и отдаёт платформе токен этой учётки.
//
// Порядок именно такой: права спрашиваются до изменения кластера, а платформа
// узнаёт о кластере последней — если она откажет, в кластере уже что-то
// создано, и повтор это переиспользует, а не задвоит.
func (uc *ConnectClusterUseCase) Execute(
	ctx context.Context,
	input ConnectClusterInput,
	confirm ConfirmFunc,
) (*entities.ConnectedCluster, error) {
	rules, err := uc.access.RequiredAccess(ctx)
	if err != nil {
		return nil, fmt.Errorf("получить требуемые права: %w", err)
	}

	target, err := uc.provisioner.Target(input.Kubeconfig, input.Context)
	if err != nil {
		return nil, err
	}

	plan := ConnectClusterPlan{
		Endpoint:       target.Endpoint,
		ServiceAccount: uc.provisioner.AccountName(),
		Rules:          rules,
	}
	agreed, err := confirm(plan)
	if err != nil {
		return nil, err
	}
	if !agreed {
		return nil, entities.ErrCancelled
	}

	credential, err := uc.provisioner.Provision(ctx, input.Kubeconfig, input.Context, rules)
	if err != nil {
		return nil, err
	}

	cluster, err := uc.registrar.Register(ctx, input.Name, *credential)
	if err != nil {
		return nil, fmt.Errorf("зарегистрировать кластер на платформе: %w", err)
	}
	return cluster, nil
}
