// Package componentreadergomod распознаёт состав Go-сервиса по его go.mod
// (TRA-539). Нет go.mod — сервис не на Go, состав пуст.
package componentreadergomod

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type Reader struct{}

func New() *Reader {
	return &Reader{}
}

func (r *Reader) Read(_ context.Context, dir string) ([]entities.Component, error) {
	path := filepath.Join(dir, "go.mod")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("чтение %s: %w", path, err)
	}
	components, err := entities.GoModuleComponents(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return components, nil
}
