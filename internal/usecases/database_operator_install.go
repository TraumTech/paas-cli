package usecases

import (
	"context"
	"fmt"
	"strings"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type InstallOperatorInput struct {
	// Engine — тип СУБД, оператор которого ставится.
	Engine string
	// Context — контекст kubeconfig; пусто означает текущий.
	Context string
	// Kubeconfig — путь к файлу; пусто означает обычное разрешение.
	Kubeconfig string
}

// InstallOperatorPlan — что команда собирается сделать в кластере владельца.
type InstallOperatorPlan struct {
	Endpoint string
	// ClusterName — под каким именем кластер подключён к платформе.
	ClusterName string
	Operator    entities.DatabaseOperator
	Objects     []entities.ManifestObject
	// ServiceAccount — учётная запись платформы, которой достанется право.
	ServiceAccount string
}

type InstallOperatorConfirmFunc func(plan InstallOperatorPlan) (bool, error)

// InstallOperatorResult — итог: имя кластера для сообщения и отчёт по объектам.
type InstallOperatorResult struct {
	ClusterName string
	Report      entities.OperatorInstallReport
}

type InstallOperatorUseCase struct {
	operators DatabaseOperatorSource
	clusters  ClusterDirectory
	installer OperatorInstaller
}

func NewInstallOperator(o DatabaseOperatorSource, c ClusterDirectory, i OperatorInstaller) *InstallOperatorUseCase {
	return &InstallOperatorUseCase{operators: o, clusters: c, installer: i}
}

// Execute ставит оператор СУБД в подключённый кластер доступом владельца.
// Платформе её доступ не передаётся: команда пользуется им только чтобы
// применить манифест и выдать учётной записи платформы одно узкое право.
//
// Кластер должен быть подключён — иначе ставить оператор не для кого: право
// достаётся учётной записи, которую заводит подключение.
func (uc *InstallOperatorUseCase) Execute(
	ctx context.Context,
	input InstallOperatorInput,
	confirm InstallOperatorConfirmFunc,
) (*InstallOperatorResult, error) {
	if strings.TrimSpace(input.Engine) == "" {
		return nil, entities.ErrEmptyEngine
	}

	op, err := uc.operators.Operator(ctx, input.Engine)
	if err != nil {
		return nil, fmt.Errorf("получить оператор от платформы: %w", err)
	}

	target, err := uc.installer.Target(input.Kubeconfig, input.Context)
	if err != nil {
		return nil, err
	}
	cluster, err := uc.connectedCluster(ctx, target.Endpoint)
	if err != nil {
		return nil, err
	}

	objects, err := uc.installer.Objects(op.Manifest)
	if err != nil {
		return nil, err
	}

	agreed, err := confirm(InstallOperatorPlan{
		Endpoint:       target.Endpoint,
		ClusterName:    cluster.Name,
		Operator:       *op,
		Objects:        objects,
		ServiceAccount: uc.installer.AccountName(),
	})
	if err != nil {
		return nil, err
	}
	if !agreed {
		return nil, entities.ErrCancelled
	}

	report, err := uc.installer.Install(ctx, input.Kubeconfig, input.Context, op)
	if err != nil {
		return nil, err
	}
	return &InstallOperatorResult{ClusterName: cluster.Name, Report: *report}, nil
}

func (uc *InstallOperatorUseCase) connectedCluster(ctx context.Context, endpoint string) (*entities.ConnectedCluster, error) {
	clusters, err := uc.clusters.ListClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("получить перечень кластеров: %w", err)
	}
	for _, cluster := range clusters {
		if strings.TrimRight(cluster.Endpoint, "/") == strings.TrimRight(endpoint, "/") {
			return &cluster, nil
		}
	}
	return nil, entities.ErrClusterNotConnected
}
