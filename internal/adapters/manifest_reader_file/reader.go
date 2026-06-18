package manifestreaderfile

import (
	"context"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// Reader читает манифест зависимостей (protocols.toml) из репозитория потребителя.
type Reader struct{}

func New() *Reader {
	return &Reader{}
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
}

type fileDependency struct {
	Name    string   `toml:"name"`
	Methods []string `toml:"methods"`
}

func (r *Reader) Read(_ context.Context, path string) (*entities.Manifest, error) {
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
		}
	}
	for _, dep := range file.Dependencies {
		manifest.Dependencies = append(manifest.Dependencies, entities.ManifestDependency{
			Name:    dep.Name,
			Methods: dep.Methods,
		})
	}
	return manifest, nil
}
