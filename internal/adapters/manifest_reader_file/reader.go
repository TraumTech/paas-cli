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
	Destination  string           `toml:"destination"`
	Dependencies []fileDependency `toml:"dependencies"`
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
	for _, dep := range file.Dependencies {
		manifest.Dependencies = append(manifest.Dependencies, entities.ManifestDependency{
			Name:    dep.Name,
			Methods: dep.Methods,
		})
	}
	return manifest, nil
}
