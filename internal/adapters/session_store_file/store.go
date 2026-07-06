package sessionstorefile

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// Store хранит токен сессии пользователя в его конфиг-директории
// (<UserConfigDir>/paas-cli/session). Директория 0700 и файл 0600 — вход
// недоступен другим пользователям машины. Запись атомарна: сначала во временный
// файл рядом с целью, затем rename — прежний вход не портится при сбое записи.
type Store struct{}

func New() *Store {
	return &Store{}
}

func (s *Store) Save(_ context.Context, token string) error {
	path, err := sessionPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("создание каталога %s: %w", dir, err)
	}

	// os.CreateTemp создаёт файл с правами 0600 — токен читается только владельцем.
	tmp, err := os.CreateTemp(dir, ".session-*.tmp")
	if err != nil {
		return fmt.Errorf("создание временного файла: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.WriteString(token + "\n"); err != nil {
		tmp.Close()
		return fmt.Errorf("запись токена сессии: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("закрытие временного файла: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("сохранение токена сессии в %s: %w", path, err)
	}
	return nil
}

func (s *Store) Load(_ context.Context) (string, error) {
	path, err := sessionPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", entities.ErrNoSession
	}
	if err != nil {
		return "", fmt.Errorf("чтение токена сессии из %s: %w", path, err)
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", entities.ErrNoSession
	}
	return token, nil
}

func (s *Store) Delete(_ context.Context) error {
	path, err := sessionPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("удаление токена сессии %s: %w", path, err)
	}
	return nil
}

func sessionPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("конфиг-директория пользователя недоступна: %w", err)
	}
	return filepath.Join(dir, "paas-cli", "session"), nil
}
