package usecases

import (
	"context"

	"github.com/TraumTech/paas-cli/internal/entities"
)

//go:generate go run go.uber.org/mock/mockgen@latest -destination=dependencies_mock_test.go -package=usecases github.com/TraumTech/paas-cli/internal/usecases ProtocolSource,ProtocolStore

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
