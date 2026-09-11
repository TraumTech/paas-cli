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
	// Topology — заявленная владельцем топология; пусто означает «предложи
	// по зонам нод».
	Topology entities.ClusterTopology
}

// ConnectClusterPlan — что команда собирается сделать в кластере владельца.
// Показывается до применения: это чужой кластер, и молча менять его нельзя.
type ConnectClusterPlan struct {
	// Endpoint — адрес кластера, к которому команда обратится.
	Endpoint string
	// ServiceAccount — имя учётной записи, которую заведёт команда.
	ServiceAccount string
	Rules          []entities.AccessRule
	// Topology — с какой топологией кластер будет объявлен, и откуда она:
	// задана владельцем или предложена по зонам нод.
	Topology         entities.ClusterTopology
	TopologyDeclared bool
	Zones            []string
	// Warnings — на что владельцу стоит взглянуть до подтверждения; не
	// останавливают: топология — его заявление.
	Warnings []string
}

// ConfirmFunc спрашивает у владельца согласие на изменение кластера. Возврат
// false означает отказ — ничего не применяется.
type ConfirmFunc func(plan ConnectClusterPlan) (bool, error)

// AlreadyConnected — кластер с таким именем уже подключён; повтор ничего не
// меняет, а владельцу показывают, что объявлено и что видно по нодам сейчас.
type AlreadyConnected struct {
	Cluster  entities.ConnectedCluster
	Proposed entities.ClusterTopology
	Zones    []string
}

type ConnectClusterUseCase struct {
	access      ClusterAccessSource
	provisioner ClusterProvisioner
	registrar   ClusterRegistrar
	directory   ClusterDirectory
}

func NewConnectCluster(a ClusterAccessSource, p ClusterProvisioner, r ClusterRegistrar, d ClusterDirectory) *ConnectClusterUseCase {
	return &ConnectClusterUseCase{access: a, provisioner: p, registrar: r, directory: d}
}

// Execute подключает кластер: спрашивает у платформы, какие права ей нужны,
// показывает владельцу что будет создано, заводит учётную запись его же
// доступом и отдаёт платформе токен этой учётки.
//
// Порядок именно такой: права спрашиваются до изменения кластера, а платформа
// узнаёт о кластере последней — если она откажет, в кластере уже что-то
// создано, и повтор это переиспользует, а не задвоит.
//
// Уже подключённый кластер возвращается как *AlreadyConnected без единого
// изменения: заявленная топология повтором не меняется.
func (uc *ConnectClusterUseCase) Execute(
	ctx context.Context,
	input ConnectClusterInput,
	confirm ConfirmFunc,
) (*entities.ConnectedCluster, *AlreadyConnected, error) {
	if input.Topology != "" {
		if err := entities.ValidateClusterTopology(input.Topology); err != nil {
			return nil, nil, err
		}
	}

	rules, err := uc.access.RequiredAccess(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("получить требуемые права: %w", err)
	}

	target, err := uc.provisioner.Target(input.Kubeconfig, input.Context)
	if err != nil {
		return nil, nil, err
	}
	proposed := entities.ProposeClusterTopology(target.Zones)

	existing, err := uc.findConnected(ctx, input.Name)
	if err != nil {
		return nil, nil, err
	}
	if existing != nil {
		return nil, &AlreadyConnected{Cluster: *existing, Proposed: proposed, Zones: target.Zones}, nil
	}

	plan := ConnectClusterPlan{
		Endpoint:         target.Endpoint,
		ServiceAccount:   uc.provisioner.AccountName(),
		Rules:            rules,
		Topology:         proposed,
		TopologyDeclared: input.Topology != "",
		Zones:            target.Zones,
	}
	if input.Topology != "" {
		plan.Topology = input.Topology
	}
	// Владелец обещает две зоны, а ноды сейчас в одной — не спорим, но говорим.
	if plan.Topology == entities.ClusterTopologyRegional && len(target.Zones) < 2 {
		plan.Warnings = append(plan.Warnings,
			"объявлен региональный кластер, но ноды сейчас в одной зоне — платформа поверит заявлению")
	}
	agreed, err := confirm(plan)
	if err != nil {
		return nil, nil, err
	}
	if !agreed {
		return nil, nil, entities.ErrCancelled
	}

	credential, err := uc.provisioner.Provision(ctx, input.Kubeconfig, input.Context, rules)
	if err != nil {
		return nil, nil, err
	}

	cluster, err := uc.registrar.Register(ctx, input.Name, *credential, plan.Topology)
	if err != nil {
		return nil, nil, fmt.Errorf("зарегистрировать кластер на платформе: %w", err)
	}
	return cluster, nil, nil
}

func (uc *ConnectClusterUseCase) findConnected(ctx context.Context, name string) (*entities.ConnectedCluster, error) {
	clusters, err := uc.directory.ListClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("получить перечень кластеров: %w", err)
	}
	for _, c := range clusters {
		if c.Name == name {
			return &c, nil
		}
	}
	return nil, nil
}
