package usecases

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/entities"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_mock_test.go -package=usecases github.com/TraumTech/paas-cli/internal/usecases ProtocolSource,ProtocolStore,CandidateReader,CompatibilitySource,VersionPublisher,ProtocolPublisher,DependencyRegistrar,ManifestReader,ServiceResolver

// ProtocolSource достаёт актуальный опубликованный контракт сервиса из платформы.
// Возвращает entities.ErrServiceNotFound / entities.ErrProtocolNotPublished,
// когда контракта нет — use case транслирует их пользователю как есть.
type ProtocolSource interface {
	FetchProtocol(ctx context.Context, serviceID string) (*entities.Protocol, error)
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

type CompatibilitySource interface {
	CheckCompatibility(ctx context.Context, serviceID string, document []byte) (*entities.CompatibilityReport, error)
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
// потребителями. На отказ платформы (нет сервиса/версии, контракт отклонён)
// возвращает ошибку с понятным сообщением от платформы.
type ProtocolPublisher interface {
	PublishProtocol(ctx context.Context, serviceID, versionID string, document []byte) (*entities.ProtocolPublication, error)
}

// DependencyRegistrar регистрирует в платформе зависимость версии потребителя от
// контракта продьюсера, прикладывая снимок этого контракта. Идемпотентен:
// повторная регистрация той же версии на того же продьюсера обновляет снимок, а
// не плодит дубль. На отказ платформы (нет версии-потребителя или продьюсера,
// снимок отклонён) возвращает ошибку с понятным сообщением от платформы.
type DependencyRegistrar interface {
	RegisterDependency(ctx context.Context, serviceID, versionID, producerServiceID string, document []byte) (*entities.Dependency, error)
}

// ManifestReader читает манифест зависимостей из файла в репозитории потребителя.
type ManifestReader interface {
	Read(ctx context.Context, path string) (*entities.Manifest, error)
}

// ServiceResolver находит id сервисов платформы по именам: манифест адресует
// продьюсеров по имени, а платформа — по id. Резолвит весь манифест одним запросом
// и возвращает карту name→id только по найденным сервисам; ненайденные имена в карту
// не попадают (вызывающий сам решает, что это ошибка).
type ServiceResolver interface {
	ResolveIDs(ctx context.Context, names []string) (map[string]string, error)
}
