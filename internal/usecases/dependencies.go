package usecases

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/entities"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_mock_test.go -package=usecases github.com/TraumTech/paas-cli/internal/usecases ProtocolSource,ProtocolStore,CandidateReader,CompatibilitySource,VersionPublisher,ProtocolPublisher,DependencyRegistrar,ManifestReader,ServiceResolver,CredentialAuthenticator,SessionInspector,SessionRevoker,SessionStore

// ProtocolSource достаёт актуальный опубликованный контракт сервиса из платформы.
// Непустой methods — контракт, суженный платформой до этих методов (CLI-09);
// narrowingSkipped — сужение для формата контракта не поддерживается, контракт
// пришёл целиком. Возвращает entities.ErrServiceNotFound /
// entities.ErrProtocolNotPublished, когда контракта нет — use case транслирует
// их пользователю как есть; отказ платформы в сужении (метод не найден в
// контракте) приходит ошибкой с её сообщением.
type ProtocolSource interface {
	FetchProtocol(ctx context.Context, serviceID string, methods []string) (protocol *entities.Protocol, narrowingSkipped bool, err error)
}

// ProtocolStore сохраняет контракт к потребителю в директорию destDir и
// возвращает итоговый путь файла. Запись атомарна: рабочий контракт не затирается
// частичным/битым результатом.
type ProtocolStore interface {
	Save(ctx context.Context, protocol *entities.Protocol, destDir string) (string, error)
}

type CandidateReader interface {
	Read(ctx context.Context, path string) ([]byte, error)
}

// CompatibilitySource сверяет контракт-кандидата (в родном виде его формата) со
// снимками потребителей сервиса на платформе, без публикации.
type CompatibilitySource interface {
	CheckCompatibility(ctx context.Context, serviceID string, format entities.ProtocolFormat, document []byte) (*entities.CompatibilityReport, error)
}

// VersionPublisher фиксирует версию сервиса в платформе по развёрнутой ревизии
// коммита. Публикация идемпотентна: повторный вызов с той же ревизией возвращает
// уже существующую версию, а не создаёт дубликат. Возвращает
// entities.ErrServiceNotFound, когда сервиса нет.
type VersionPublisher interface {
	PublishVersion(ctx context.Context, serviceID, commitRevision string) (*entities.Version, error)
}

// ProtocolPublisher публикует контракт под версией сервиса в платформе и
// возвращает итог: к какой версии привязан протокол и его совместимость с
// потребителями. Формат доносится до платформы как есть; глубокую проверку
// документа по формату делает она. На отказ платформы (нет сервиса/версии,
// контракт отклонён) возвращает ошибку с понятным сообщением от платформы.
type ProtocolPublisher interface {
	PublishProtocol(ctx context.Context, serviceID, versionID string, format entities.ProtocolFormat, document []byte) (*entities.ProtocolPublication, error)
}

// DependencyRegistrar регистрирует в платформе зависимость версии потребителя от
// контракта продьюсера, прикладывая снимок этого контракта в родном виде его
// формата (PRT-19). Идемпотентен: повторная регистрация той же версии на того же
// продьюсера обновляет снимок, а не плодит дубль. На отказ платформы (нет
// версии-потребителя или продьюсера, снимок отклонён) возвращает ошибку с
// понятным сообщением от платформы.
type DependencyRegistrar interface {
	RegisterDependency(ctx context.Context, in DependencyRegistration) (*entities.Dependency, error)
}

// DependencyRegistration — одна регистрация в платформе. Структурой, а не
// позиционными аргументами: Methods и Waived — соседние перечни строк,
// перепутать их местами было бы молча.
type DependencyRegistration struct {
	ServiceID         string
	VersionID         string
	ProducerServiceID string
	Format            entities.ProtocolFormat
	Document          []byte
	// Methods — используемые методы продьюсера; пусто — зависимость от всего снимка.
	Methods []string
	// Waived — атрибуты продьюсера, от которых потребитель отказался (PRT-26).
	Waived            []string
	SupersedePrevious bool
}

// ManifestReader читает манифест зависимостей из файла в репозитории потребителя.
type ManifestReader interface {
	Read(ctx context.Context, path string) (*entities.Manifest, error)
}

// CredentialAuthenticator обменивает учётные данные пользователя на сессию у
// identity-провайдера платформы. Возвращает entities.ErrInvalidCredentials, когда
// провайдер не признал учётные данные (без уточнения, что именно неверно), — use
// case транслирует её пользователю как есть.
type CredentialAuthenticator interface {
	Authenticate(ctx context.Context, email, password string) (*entities.UserSession, error)
}

// SessionInspector проверяет токен сессии у identity-провайдера и возвращает,
// кому сессия принадлежит. Возвращает entities.ErrSessionExpired, когда токен
// недействителен (истёк или отозван).
type SessionInspector interface {
	Inspect(ctx context.Context, token string) (*entities.UserSession, error)
}

// SessionRevoker завершает сессию по её токену у identity-провайдера. Уже
// недействительный токен ошибкой не считается: цель — чтобы сессии не стало.
type SessionRevoker interface {
	Revoke(ctx context.Context, token string) error
}

// SessionStore хранит токен сессии локально у пользователя, недоступно другим
// пользователям машины. Load возвращает entities.ErrNoSession, когда сохранённого
// входа нет; Delete от отсутствия входа не падает.
type SessionStore interface {
	Save(ctx context.Context, token string) error
	Load(ctx context.Context) (string, error)
	Delete(ctx context.Context) error
}

// ServiceResolver находит id сервисов платформы по именам: манифест адресует
// продьюсеров по имени, а платформа — по id. Резолвит весь манифест одним запросом
// и возвращает карту name→id только по найденным сервисам; ненайденные имена в карту
// не попадают (вызывающий сам решает, что это ошибка).
type ServiceResolver interface {
	ResolveIDs(ctx context.Context, names []string) (map[string]string, error)
}
