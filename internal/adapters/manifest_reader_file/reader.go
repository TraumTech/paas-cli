package manifestreaderfile

import (
	"context"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"

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
	var unified fileManifest
	if err := toml.Unmarshal(data, &unified); err != nil {
		return "", fmt.Errorf("манифест %s не разобран как TOML: %w", unifiedManifestPath, err)
	}
	if unified.Service == nil && len(unified.Dependencies) == 0 {
		if _, err := os.Stat(legacyManifestPath); err == nil {
			return "", fmt.Errorf("%s не содержит секций манифеста ([service], dependencies) — перенесите их из %s: paas.toml теперь единственный манифест репозитория",
				unifiedManifestPath, legacyManifestPath)
		}
	}
	return unifiedManifestPath, nil
}

// fileManifest — транспортная форма манифеста (TOML); маппинг в доменный
// entities.Manifest живёт только здесь, в адаптере.
type fileManifest struct {
	Service      *fileService     `toml:"service"`
	Destination  string           `toml:"destination"`
	Dependencies []fileDependency `toml:"dependencies"`
}

type fileService struct {
	Name     string `toml:"name"`
	Contract string `toml:"contract"`
	Format   string `toml:"format"`
}

type fileDependency struct {
	Name       string   `toml:"name"`
	Methods    []string `toml:"methods"`
	Waived     []string `toml:"waived"`
	Attributes []string `toml:"attributes"`
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
	var file fileManifest
	if err := toml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("манифест %s не разобран как TOML: %w", path, err)
	}

	manifest := &entities.Manifest{Destination: file.Destination}
	if file.Service != nil {
		manifest.Service = &entities.ManifestService{
			Name:     file.Service.Name,
			Contract: file.Service.Contract,
			Format:   file.Service.Format,
		}
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
