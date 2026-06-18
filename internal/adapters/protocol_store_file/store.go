package protocolstorefile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// Store пишет контракт в <destDir>/<service-name>/openapi.json атомарно: сначала
// во временный файл рядом с целью, затем rename. Если запись падает — ранее
// полученный рабочий контракт остаётся нетронутым.
type Store struct{}

func New() *Store {
	return &Store{}
}

func (s *Store) Save(_ context.Context, protocol *entities.Protocol, destDir string) (string, error) {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, protocol.Document, "", "  "); err != nil {
		return "", fmt.Errorf("форматирование контракта: %w", err)
	}
	pretty.WriteByte('\n')

	dir := filepath.Join(destDir, protocol.ServiceName)
	destPath := filepath.Join(dir, entities.ProtocolFileName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("создание каталога %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".paas-protocol-*.tmp")
	if err != nil {
		return "", fmt.Errorf("создание временного файла: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(pretty.Bytes()); err != nil {
		tmp.Close()
		return "", fmt.Errorf("запись контракта: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("закрытие временного файла: %w", err)
	}
	if err := os.Rename(tmpName, destPath); err != nil {
		return "", fmt.Errorf("публикация контракта в %s: %w", destPath, err)
	}
	return destPath, nil
}
