package entities

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
)
