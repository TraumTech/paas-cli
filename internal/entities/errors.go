package entities

import "fmt"

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
	// Форма без образа не даст платформе сгенерировать манифесты — отказываем
	// до обращения к сети (DEP-02).
	ErrFormRequiresImage = newDomainError("paas.toml найден — укажите адрес образа флагом --image")

	ErrUnknownEnvironment = newDomainError("окружение должно быть одним из: dev, stage, prod")
	// Секция [env.<имя>] в манифесте для окружения, которого у платформы нет.
	ErrUnknownFormEnvironment = newDomainError("секция [env.<окружение>] в paas.toml указывает на неизвестное окружение: допустимы default, dev, stage, prod")

	ErrMethodsUnsupportedForFormat = newDomainError("сужение до методов для контракта этого формата не поддерживается — уберите methods у этой зависимости")

	ErrManifestNoDependencies   = newDomainError("в манифесте не объявлено ни одной зависимости")
	ErrManifestDependencyNoName = newDomainError("у зависимости в манифесте не указано имя сервиса")

	ErrManifestNoService         = newDomainError("манифест не объявляет текущий сервис: добавьте секцию [service] с именем сервиса (name)")
	ErrManifestServiceNoName     = newDomainError("в секции [service] манифеста не указано имя сервиса (name)")
	ErrManifestServiceNoContract = newDomainError("в секции [service] манифеста не указан путь к контракту (contract)")

	ErrEmptyCredentials   = newDomainError("укажите e-mail и пароль")
	ErrInvalidCredentials = newDomainError("войти не удалось: проверьте учётные данные")
	ErrNoSession          = newDomainError("вход не выполнен — выполните `paas-cli auth login`")
	ErrSessionExpired     = newDomainError("сессия истекла или отозвана — войдите заново: `paas-cli auth login`")

	ErrCancelled           = newDomainError("отменено — в кластере ничего не изменено")
	ErrEmptyClusterName    = newDomainError("укажите имя кластера: --name")
	ErrNoKubeContext       = newDomainError("не удалось определить кластер: в kubeconfig нет активного контекста")
	ErrClusterAccessDenied = newDomainError("вашего доступа не хватает, чтобы выдать права платформе — нужен доступ уровня администратора кластера")

	ErrLoginRequired        = newDomainError("платформа требует вход: выполните `paas-cli auth login` или задайте токен доступа в PAAS_API_TOKEN")
	ErrTokenRejected        = newDomainError("токен из PAAS_API_TOKEN не принят платформой — он отозван, просрочен или неверен")

	// Браузерный вход (AUTH-22).
	ErrAuthorizationDenied  = newDomainError("вход не подтверждён в браузере — ничего не сохранено")
	ErrAuthorizationTimeout = newDomainError("подтверждение из браузера не пришло — повторите вход")
	ErrBrowserUnavailable   = newDomainError("не удалось открыть браузер — войдите паролем: `paas-cli auth login --password`")
	// Личный токен предъявляется платформе, а не провайдеру: истёкший и
	// отозванный она одинаково не принимает.
	ErrPersonalTokenRejected = newDomainError("личный токен истёк или отозван — войдите заново: `paas-cli auth login`")
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
