package sessionstorefile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// Store хранит вход пользователя в его конфиг-директории
// (<UserConfigDir>/paas-cli/session). Директория 0700 и файл 0600 — вход
// недоступен другим пользователям машины. Запись атомарна: сначала во временный
// файл рядом с целью, затем rename — прежний вход не портится при сбое записи.
//
// Формат — JSON: кроме секрета нужно помнить, чем он предъявляется и где
// проверяется (сессия провайдера или личный токен платформы, AUTH-22). Вход,
// сохранённый прежними версиями CLI, — голая строка токена сессии; такой файл
// читается как сессия, поэтому обновление CLI не выкидывает пользователя.
type Store struct{}

func New() *Store {
	return &Store{}
}

func (s *Store) Save(_ context.Context, credential entities.Credential) error {
	path, err := sessionPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("создание каталога %s: %w", dir, err)
	}

	payload, err := json.Marshal(credential)
	if err != nil {
		return fmt.Errorf("подготовка сохраняемого входа: %w", err)
	}

	// os.CreateTemp создаёт файл с правами 0600 — токен читается только владельцем.
	tmp, err := os.CreateTemp(dir, ".session-*.tmp")
	if err != nil {
		return fmt.Errorf("создание временного файла: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(append(payload, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("запись входа: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("закрытие временного файла: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("сохранение входа в %s: %w", path, err)
	}
	return nil
}

func (s *Store) Load(_ context.Context) (*entities.Credential, error) {
	path, err := sessionPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, entities.ErrNoSession
	}
	if err != nil {
		return nil, fmt.Errorf("чтение входа из %s: %w", path, err)
	}
	raw := strings.TrimSpace(string(data))
	if raw == "" {
		return nil, entities.ErrNoSession
	}

	var credential entities.Credential
	if err := json.Unmarshal([]byte(raw), &credential); err != nil {
		// Вход прежних версий CLI — голый токен сессии.
		return &entities.Credential{Kind: entities.CredentialSession, Token: raw}, nil
	}
	if credential.Token == "" {
		return nil, entities.ErrNoSession
	}
	if credential.Kind == "" {
		credential.Kind = entities.CredentialSession
	}
	return &credential, nil
}

func (s *Store) Delete(_ context.Context) error {
	path, err := sessionPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("удаление входа %s: %w", path, err)
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
