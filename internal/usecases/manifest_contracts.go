package usecases

import (
	"context"
	"fmt"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// manifestContracts — контракты манифеста, прочитанные и проверенные до похода
// на платформу: общий пролог публикации протоколов и досрочной проверки
// (CLI-23). Порядок contracts повторяет protocols.
type manifestContracts struct {
	manifest  *entities.Manifest
	protocols []entities.ManifestProtocol
	contracts []*entities.CandidateContract
}

// loadManifestContracts читает манифест и все объявленные контракты. Все записи
// проверяются до возврата: опечатка в любой — ошибка сразу, а не перечень,
// обработанный наполовину.
func loadManifestContracts(ctx context.Context, manifests ManifestReader, reader CandidateReader, manifestPath string) (*manifestContracts, error) {
	manifest, err := manifests.Read(ctx, manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	protocols, err := manifest.RequireProtocols()
	if err != nil {
		return nil, err
	}
	contracts := make([]*entities.CandidateContract, len(protocols))
	for i, p := range protocols {
		format, err := entities.ParseProtocolFormat(p.Format)
		if err != nil {
			return nil, protocolError(p.Name, err)
		}
		document, err := reader.Read(ctx, contractPath(manifestPath, p.Contract))
		if err != nil {
			return nil, protocolError(p.Name, fmt.Errorf("read contract: %w", err))
		}
		contracts[i] = &entities.CandidateContract{Format: format, Document: document}
		if err := contracts[i].Validate(); err != nil {
			return nil, protocolError(p.Name, fmt.Errorf("validate contract: %w", err))
		}
	}
	return &manifestContracts{manifest: manifest, protocols: protocols, contracts: contracts}, nil
}

// protocolError подписывает ошибку именем записи перечня; у записи прежней формы
// (без имени) текст остаётся прежним — манифесты с одним контрактом видят те же
// сообщения, что раньше.
func protocolError(name string, err error) error {
	if name == "" {
		return err
	}
	return fmt.Errorf("протокол %q: %w", name, err)
}

// checkOrphanedProtocols — гейт исчезнувшего протокола (CLI-23): протокол,
// живущий в реестре, но пропавший из манифеста (в том числе после
// переименования имени), для своих потребителей — ломающее изменение. С
// потребителями — ошибка, без потребителей — имя возвращается предупреждением.
// До «зависимости от именованного протокола» (roadmap EPIC-03) все
// зарегистрированные потребители — потребители протокола по умолчанию, поэтому
// потребители спрашиваются только для него.
func checkOrphanedProtocols(ctx context.Context, registry RegistryDirectory, serviceID string, declared []entities.ManifestProtocol) ([]string, error) {
	published, err := registry.ListProtocols(ctx, serviceID)
	if err != nil {
		return nil, fmt.Errorf("list protocols: %w", err)
	}
	orphans := entities.OrphanedProtocols(published, declared)
	if len(orphans) == 0 {
		return nil, nil
	}
	warnings := make([]string, 0, len(orphans))
	for _, orphan := range orphans {
		if orphan.Name == entities.DefaultProtocolName {
			consumers, err := registry.ListConsumers(ctx, serviceID)
			if err != nil {
				return nil, fmt.Errorf("list consumers: %w", err)
			}
			if len(consumers) > 0 {
				return nil, &entities.OrphanedProtocolError{Name: orphan.Name, Consumers: consumers}
			}
		}
		warnings = append(warnings, orphan.Name)
	}
	return warnings, nil
}
