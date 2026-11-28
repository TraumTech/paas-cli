package manifestreaderfile

import (
	"context"
	"fmt"
	"os"

	servicefile "github.com/TraumTech/paas-cli/internal/adapters/service_file"
	"github.com/TraumTech/paas-cli/internal/entities"
)

// Reader читает манифест репозитория. Единственный манифест — paas.toml
// (CLI-22); protocols.toml остаётся переходным именем и снимется отдельным
// шагом после переезда всех репозиториев.
type Reader struct{}

func New() *Reader {
	return &Reader{}
}

// Имена манифеста: новое и переходное.
const (
	unifiedManifestPath = "paas.toml"
	legacyManifestPath  = "protocols.toml"
)

// Resolve выбирает файл манифеста, когда путь не задан явно: paas.toml, а при
// его отсутствии — переходный protocols.toml. Полупереехавший репозиторий
// (paas.toml есть, но секции манифеста остались в protocols.toml) — явная
// ошибка, а не молчаливое чтение старого файла: иначе правки в новом файле
// молча не действовали бы.
func resolve(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	if _, err := os.Stat(unifiedManifestPath); err != nil {
		return legacyManifestPath, nil
	}
	data, err := os.ReadFile(unifiedManifestPath)
	if err != nil {
		return "", fmt.Errorf("чтение манифеста %s: %w", unifiedManifestPath, err)
	}
	unified, err := servicefile.Decode(unifiedManifestPath, data)
	if err != nil {
		return "", err
	}
	if unified.Service == nil && len(unified.Protocols) == 0 && len(unified.Dependencies) == 0 {
		if _, err := os.Stat(legacyManifestPath); err == nil {
			return "", fmt.Errorf("%s не содержит секций манифеста ([service], dependencies) — перенесите их из %s: paas.toml теперь единственный манифест репозитория",
				unifiedManifestPath, legacyManifestPath)
		}
	}
	return unifiedManifestPath, nil
}

func (r *Reader) Read(_ context.Context, path string) (*entities.Manifest, error) {
	path, err := resolve(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("чтение манифеста %s: %w", path, err)
	}
	file, err := servicefile.Decode(path, data)
	if err != nil {
		return nil, err
	}

	manifest := &entities.Manifest{Destination: file.Destination}
	if file.Service != nil {
		manifest.Service = &entities.ManifestService{
			Name:     file.Service.Name,
			Contract: file.Service.Contract,
			Format:   file.Service.Format,
		}
	}
	for _, p := range file.Protocols {
		manifest.Protocols = append(manifest.Protocols, entities.ManifestProtocol{
			Name:     p.Name,
			Contract: p.Contract,
			Format:   p.Format,
		})
	}
	for _, dep := range file.Dependencies {
		manifest.Dependencies = append(manifest.Dependencies, entities.ManifestDependency{
			Name:       dep.Name,
			Methods:    dep.Methods,
			Waived:     dep.Waived,
			Attributes: dep.Attributes,
		})
	}
	return manifest, nil
}
