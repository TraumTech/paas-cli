package entities

import (
	"fmt"
	"strings"
)

type DomainError struct {
	message string
}

func newDomainError(message string) *DomainError {
	return &DomainError{message: message}
}

func (e *DomainError) Error() string {
	return e.message
}

var (
	ErrServiceNotFound      = newDomainError("сервис не найден")
	ErrProtocolNotPublished = newDomainError("контракт сервиса ещё не опубликован")
	ErrEmptyProtocol        = newDomainError("контракт пуст")
	ErrInvalidProtocol      = newDomainError("ответ не похож на OpenAPI-контракт")
	ErrEmptyCommitRevision  = newDomainError("ревизия коммита не указана")
	ErrNoMethodsSelected    = newDomainError("не указан ни один метод")

	ErrMethodsUnsupportedForGRPC = newDomainError("сужение до методов для gRPC-контракта пока не поддерживается — уберите methods у этой зависимости")

	ErrManifestNoDependencies   = newDomainError("в манифесте не объявлено ни одной зависимости")
	ErrManifestDependencyNoName = newDomainError("у зависимости в манифесте не указано имя сервиса")

	ErrManifestNoService         = newDomainError("манифест не объявляет текущий сервис: добавьте секцию [service] с именем сервиса (name)")
	ErrManifestServiceNoName     = newDomainError("в секции [service] манифеста не указано имя сервиса (name)")
	ErrManifestServiceNoContract = newDomainError("в секции [service] манифеста не указан путь к контракту (contract)")
)

// UnsupportedProtocolFormatError сообщает, какой формат из манифеста CLI не
// поддерживает, и перечисляет поддерживаемые — чтобы опечатка не ушла на
// платформу публикацией не того типа.
type UnsupportedProtocolFormatError struct {
	Name string
}

func (e *UnsupportedProtocolFormatError) Error() string {
	return fmt.Sprintf("формат протокола %q не поддерживается (поддерживаются: %s, %s)", e.Name, ProtocolFormatOpenAPI, ProtocolFormatGRPC)
}

// UnknownMethodsError сообщает, какие именно запрошенные методы не нашлись в
// контракте, — чтобы пользователь видел причину, а не молча получал неполный срез.
type UnknownMethodsError struct {
	Methods []string
}

func (e *UnknownMethodsError) Error() string {
	return fmt.Sprintf("методы не найдены в контракте: %s", strings.Join(e.Methods, ", "))
}
