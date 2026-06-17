package entities

import (
	"fmt"
	"strings"
)

// DefaultDestination — директория для контрактов, когда манифест не задаёт свою.
const DefaultDestination = "protocols"

// Manifest — декларация зависимостей репозитория-потребителя от контрактов сервисов
// платформы: что тянуть и куда. Источник истины о составе зависимостей, который
// читает команда синхронизации; воспроизводимость даёт git (полученные снимки
// коммитятся), поэтому манифест перечисляет сервисы, а не версии продьюсера.
type Manifest struct {
	Destination  string
	Dependencies []ManifestDependency
}

// ManifestDependency — одна объявленная зависимость: контракт сервиса-продьюсера по
// имени. Methods — необязательное сужение контракта до перечисленных методов
// (operationId); пусто — берётся контракт целиком.
type ManifestDependency struct {
	Name    string
	Methods []string
}

// EffectiveDestination — директория для раскладки контрактов: явная из манифеста,
// иначе значение по умолчанию.
func (m *Manifest) EffectiveDestination() string {
	if strings.TrimSpace(m.Destination) == "" {
		return DefaultDestination
	}
	return m.Destination
}

// Validate проверяет, что манифест осмыслен: есть хотя бы одна зависимость, у каждой
// непустое имя и имена не повторяются — чтобы прогон не оказался молчаливо пустым и
// не тянул один сервис дважды.
func (m *Manifest) Validate() error {
	if len(m.Dependencies) == 0 {
		return ErrManifestNoDependencies
	}
	seen := make(map[string]struct{}, len(m.Dependencies))
	for _, dep := range m.Dependencies {
		if strings.TrimSpace(dep.Name) == "" {
			return ErrManifestDependencyNoName
		}
		if _, dup := seen[dep.Name]; dup {
			return &ManifestDuplicateError{Name: dep.Name}
		}
		seen[dep.Name] = struct{}{}
	}
	return nil
}

// ManifestDuplicateError сообщает, какой сервис объявлен в манифесте повторно.
type ManifestDuplicateError struct {
	Name string
}

func (e *ManifestDuplicateError) Error() string {
	return fmt.Sprintf("сервис %q объявлен в манифесте повторно", e.Name)
}
