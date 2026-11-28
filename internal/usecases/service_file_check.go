package usecases

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// CheckServiceFileUseCase проверяет файл сервиса до пуша (TRA-556): локально —
// теми же правилами, что публикация сборки, и, если вход выполнен, сверяет
// ссылки файла с заведённым в организации. На платформе ничего не меняет.
type CheckServiceFileUseCase struct {
	manifests ManifestReader
	forms     FormReader
	resolver  ServiceResolver
	inventory InventorySource
	// signedIn — есть ли чем ходить на платформу; без входа сверка пропускается.
	signedIn bool
}

func NewCheckServiceFile(manifests ManifestReader, forms FormReader, resolver ServiceResolver, inventory InventorySource, signedIn bool) *CheckServiceFileUseCase {
	return &CheckServiceFileUseCase{manifests: manifests, forms: forms, resolver: resolver, inventory: inventory, signedIn: signedIn}
}

func (uc *CheckServiceFileUseCase) Execute(ctx context.Context, in CheckServiceFileInput) (*CheckServiceFileResult, error) {
	if _, err := os.Stat(in.Path); errors.Is(err, fs.ErrNotExist) {
		dir, absErr := filepath.Abs(filepath.Dir(in.Path))
		if absErr != nil {
			dir = filepath.Dir(in.Path)
		}
		return nil, &entities.ServiceFileMissingError{Expected: filepath.Base(in.Path), Dir: dir}
	}
	manifest, err := uc.manifests.Read(ctx, in.Path)
	if err != nil {
		return nil, err
	}
	name, err := manifest.ServiceName()
	if err != nil {
		return nil, err
	}
	protocol, err := buildProtocol(manifest)
	if err != nil {
		return nil, err
	}
	if len(manifest.Dependencies) > 0 {
		if err := manifest.Validate(); err != nil {
			return nil, err
		}
	}
	declaration, err := uc.forms.Read(ctx, in.Path)
	if err != nil {
		return nil, err
	}
	if declaration != nil {
		if err := declaration.Validate(); err != nil {
			return nil, err
		}
	}

	result := &CheckServiceFileResult{Path: in.Path, Service: name}
	// Контракт часто собирается в CI и в рабочей копии отсутствует — это
	// замечание, а не ошибка.
	if protocol != nil {
		if _, err := os.Stat(protocol.Contract); errors.Is(err, fs.ErrNotExist) {
			result.Notes = append(result.Notes, fmt.Sprintf("контракт %s не найден: если он собирается в CI — всё в порядке, иначе поправьте путь в contract", protocol.Contract))
		}
	}
	if !uc.signedIn {
		result.PlatformSkipped = true
		return result, nil
	}
	ids, err := uc.resolver.ResolveIDs(ctx, []string{name})
	if err != nil {
		return nil, fmt.Errorf("сверка с платформой: %w", err)
	}
	if _, ok := ids[name]; !ok {
		result.Missing = append(result.Missing, entities.MissingReference{
			What:   "сервис " + name,
			Where:  "name в [service]",
			Remedy: "сервисы заводятся на " + entities.PlatformUIURL,
		})
	}
	if declaration != nil {
		inventory, err := uc.inventory.Inventory(ctx)
		if err != nil {
			return nil, fmt.Errorf("сверка с платформой: %w", err)
		}
		result.Missing = append(result.Missing, declaration.MissingReferences(*inventory)...)
	}
	return result, nil
}
