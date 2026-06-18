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
//
// Service — обязательная самодекларация репозитория: какой сервис он представляет и
// (для публикации) где его собственный контракт. Каждый манифест объявляет свой
// сервис, поэтому секция нужна и потребителю (sync), и владельцу (publish).
type Manifest struct {
	Service      *ManifestService
	Destination  string
	Dependencies []ManifestDependency
}

// ManifestService — самодекларация репозитория: имя сервиса на платформе и путь к
// его собственному контракту (относительно манифеста). Contract заполняет только
// репозиторий-владелец, который публикует протокол; чистому потребителю он не нужен.
// Версию не держим — она эфемерна, привязана к конкретной выкатке.
type ManifestService struct {
	Name     string
	Contract string
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

// Validate проверяет, что манифест осмыслен: объявлен текущий сервис (секция
// [service] с непустым именем), есть хотя бы одна зависимость, у каждой непустое имя
// и имена не повторяются — чтобы прогон не оказался молчаливо пустым и не тянул один
// сервис дважды. Контракт сервиса здесь не требуется (нужен только при публикации,
// см. RequireService) — чистый потребитель своего контракта не имеет.
func (m *Manifest) Validate() error {
	if m.Service == nil {
		return ErrManifestNoService
	}
	if strings.TrimSpace(m.Service.Name) == "" {
		return ErrManifestServiceNoName
	}
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

// RequireService возвращает самодекларацию текущего сервиса или понятную ошибку,
// если её нет либо она неполна. Нужна owner-командам, которые берут имя сервиса и
// путь к собственному контракту из манифеста, а не из аргументов.
func (m *Manifest) RequireService() (*ManifestService, error) {
	if m.Service == nil {
		return nil, ErrManifestNoService
	}
	if strings.TrimSpace(m.Service.Name) == "" {
		return nil, ErrManifestServiceNoName
	}
	if strings.TrimSpace(m.Service.Contract) == "" {
		return nil, ErrManifestServiceNoContract
	}
	return m.Service, nil
}

// ManifestDuplicateError сообщает, какой сервис объявлен в манифесте повторно.
type ManifestDuplicateError struct {
	Name string
}

func (e *ManifestDuplicateError) Error() string {
	return fmt.Sprintf("сервис %q объявлен в манифесте повторно", e.Name)
}
