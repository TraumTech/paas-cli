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
// Format — формат этого контракта каноническим именем платформы; пусто — OpenAPI,
// чтобы существующие манифесты работали без изменений (см. ParseProtocolFormat).
// Версию не держим — она эфемерна, привязана к конкретной выкатке.
type ManifestService struct {
	Name     string
	Contract string
	Format   string
}

// ManifestDependency — одна объявленная зависимость: контракт сервиса-продьюсера по
// имени. Methods — необязательное сужение контракта до перечисленных методов,
// заданных идентичностью формата (HTTP-паттерн у OpenAPI, package.Service/Method
// у gRPC); пусто — берётся контракт целиком.
//
// Waived — атрибуты продьюсера, от которых мы отказались (PRT-26):
// "полное.имя.Типа.поле", у вокабуляра — "полное.имя.Типа.ЗНАЧЕНИЕ". Перечень
// не обязан быть полным: по умолчанию используются все атрибуты, а объявляем мы
// только отпущенное — поэтому он короткий и не гниёт.
//
// Attributes — используемые атрибуты внутри объявленных методов (PRT-29),
// идентичностью атрибута формата ("GET /services#response.200.name" у OpenAPI):
// снимок сужается до них, метод без объявленных атрибутов едет целиком. В
// отличие от Waived этот перечень управляет снимком, а снимок — кодогенерацией,
// поэтому использовать необъявленное физически нельзя и перечень не гниёт.
// Требует объявленных Methods.
type ManifestDependency struct {
	Name       string
	Methods    []string
	Waived     []string
	Attributes []string
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
		if len(dep.Attributes) > 0 && len(dep.Methods) == 0 {
			return &ManifestAttributesWithoutMethodsError{Name: dep.Name}
		}
		seen[dep.Name] = struct{}{}
	}
	return nil
}

// ServiceName возвращает имя текущего сервиса из самодекларации или понятную ошибку,
// если секции нет либо имя пустое. Контракт здесь не требуется — он нужен только
// owner-команде публикации протокола (см. RequireService); фиксация версии и
// регистрация зависимости берут из манифеста лишь имя своего сервиса.
func (m *Manifest) ServiceName() (string, error) {
	if m.Service == nil {
		return "", ErrManifestNoService
	}
	if strings.TrimSpace(m.Service.Name) == "" {
		return "", ErrManifestServiceNoName
	}
	return m.Service.Name, nil
}

// RequireService возвращает самодекларацию текущего сервиса вместе с контрактом или
// понятную ошибку, если её нет либо она неполна. Нужна команде публикации протокола,
// которая берёт из манифеста и имя сервиса, и путь к собственному контракту.
func (m *Manifest) RequireService() (*ManifestService, error) {
	if _, err := m.ServiceName(); err != nil {
		return nil, err
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

// ManifestAttributesWithoutMethodsError — у зависимости объявлены attributes без
// methods: атрибут живёт внутри объявленного метода (PRT-29).
type ManifestAttributesWithoutMethodsError struct {
	Name string
}

func (e *ManifestAttributesWithoutMethodsError) Error() string {
	return fmt.Sprintf("у зависимости %q объявлены attributes без methods: атрибуты объявляются внутри объявленных методов", e.Name)
}
