package usecases

import (
	"context"
	"strings"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type InstallLogCollectorInput struct {
	// Key — ключ приёма логов этого кластера; его выдаёт платформа.
	Key string
	// PushURL — адрес приёма; пусто означает адрес платформы.
	PushURL string
	// Context — контекст kubeconfig; пусто означает текущий.
	Context string
	// Kubeconfig — путь к файлу; пусто означает обычное разрешение.
	Kubeconfig string
}

// InstallLogCollectorPlan — что команда собирается сделать в кластере.
type InstallLogCollectorPlan struct {
	Endpoint string
	PushURL  string
	Objects  []entities.ManifestObject
}

type InstallLogCollectorConfirmFunc func(plan InstallLogCollectorPlan) (bool, error)

type InstallLogCollectorResult struct {
	Endpoint string
	Report   entities.OperatorInstallReport
}

type InstallLogCollectorUseCase struct {
	installer LogCollectorInstaller
}

func NewInstallLogCollector(i LogCollectorInstaller) *InstallLogCollectorUseCase {
	return &InstallLogCollectorUseCase{installer: i}
}

// Execute ставит в кластер сборщик, который шлёт вывод сервисов платформе
// (TRA-559). Платформа в кластер за выводом не ходит: без сборщика вкладка
// «Вывод» у сервисов этого кластера пуста.
func (uc *InstallLogCollectorUseCase) Execute(
	ctx context.Context,
	input InstallLogCollectorInput,
	confirm InstallLogCollectorConfirmFunc,
) (*InstallLogCollectorResult, error) {
	collector := entities.LogCollector{
		Key:     strings.TrimSpace(input.Key),
		PushURL: strings.TrimSpace(input.PushURL),
	}
	if collector.Key == "" {
		return nil, entities.ErrEmptyLogKey
	}
	if collector.PushURL == "" {
		collector.PushURL = entities.DefaultLogPushURL
	}

	target, err := uc.installer.Target(input.Kubeconfig, input.Context)
	if err != nil {
		return nil, err
	}
	objects, err := uc.installer.Objects(collector)
	if err != nil {
		return nil, err
	}

	agreed, err := confirm(InstallLogCollectorPlan{Endpoint: target.Endpoint, PushURL: collector.PushURL, Objects: objects})
	if err != nil {
		return nil, err
	}
	if !agreed {
		return nil, entities.ErrCancelled
	}

	report, err := uc.installer.Install(ctx, input.Kubeconfig, input.Context, collector)
	if err != nil {
		return nil, err
	}
	return &InstallLogCollectorResult{Endpoint: target.Endpoint, Report: *report}, nil
}
