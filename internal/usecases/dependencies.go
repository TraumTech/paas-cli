package usecases

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/entities"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_mock_test.go -package=usecases github.com/TraumTech/paas-cli/internal/usecases ProtocolSource,ProtocolStore,CandidateReader,CompatibilitySource,VersionPublisher,ProtocolPublisher,DependencyRegistrar,ManifestReader,FormReader,ServiceResolver,CredentialAuthenticator,SessionInspector,SessionRevoker,SessionStore,BrowserAuthorizer,PersonalTokenDirectory

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
	// PublishVersion фиксирует версию ревизии в окружении (DEP-08).
	PublishVersion(ctx context.Context, serviceID, environment, commitRevision, branch, image string, form *entities.VersionForm) (*entities.Version, error)
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

// FormReader читает объявление формы из paas.toml (DEP-02). Отсутствие файла —
// штатная ветка (nil, nil): версия публикуется без формы. Значения окружений
// (DEP-14/15) приходят как объявлены — разрешает их use case, зная окружение
// публикуемой версии.
type FormReader interface {
	Read(ctx context.Context, path string) (*entities.FormDeclaration, error)
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

// SessionStore хранит вход локально у пользователя, недоступно другим
// пользователям машины. Load возвращает entities.ErrNoSession, когда сохранённого
// входа нет; Delete от отсутствия входа не падает.
type SessionStore interface {
	Save(ctx context.Context, credential entities.Credential) error
	Load(ctx context.Context) (*entities.Credential, error)
	Delete(ctx context.Context) error
}

// BrowserAuthorizer проводит выдачу личного токена через браузер (AUTH-22):
// открывает страницу подтверждения платформы и дожидается, пока она вернёт
// выпущенный токен на локальный адрес. Возвращает entities.ErrAuthorizationDenied,
// если пользователь отказал, и ошибку ожидания, если подтверждение не пришло.
type BrowserAuthorizer interface {
	Authorize(ctx context.Context) (*entities.Credential, error)
}

// PersonalTokenDirectory — личные токены пользователя на платформе (AUTH-20/21).
// CLI обращается к нему тем же входом, который проверяет: живой ли он и какой
// именно токен предъявлен. Возвращает entities.ErrPersonalTokenRejected, когда
// платформа вход не приняла.
type PersonalTokenDirectory interface {
	List(ctx context.Context) ([]entities.PersonalToken, error)
	Revoke(ctx context.Context, tokenID string) error
}

// ServiceResolver находит id сервисов платформы по именам: манифест адресует
// продьюсеров по имени, а платформа — по id. Резолвит весь манифест одним запросом
// и возвращает карту name→id только по найденным сервисам; ненайденные имена в карту
// не попадают (вызывающий сам решает, что это ошибка).
type ServiceResolver interface {
	ResolveIDs(ctx context.Context, names []string) (map[string]string, error)
}
