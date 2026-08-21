package usecases

import (
	"time"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type FetchProtocolInput struct {
	ServiceID string
	// Destination — базовая директория для контрактов; конкретный файл внутри
	// формирует хранилище из имени сервиса.
	Destination string
	// Methods — идентичности методов, которые нужно оставить в контракте
	// (HTTP-паттерн у OpenAPI, package.Service/Method у gRPC). Пусто — получаем
	// контракт целиком; иначе платформа обрезает контракт до них ещё до отдачи
	// (CLI-09).
	Methods []string
	// Attributes — идентичности атрибутов, которые нужно оставить внутри методов
	// ("GET /services#response.200.name" у OpenAPI); срез выполняет платформа
	// (PRT-29). Требует непустого Methods.
	Attributes []string
}

// FetchProtocolResult — итог получения протокола для отчёта пользователю.
// NarrowingSkipped — у зависимости объявлены methods, но сужение для её формата
// не поддерживается: контракт принесён целиком, отчёт это отражает.
// AttributeNarrowingSkipped — то же про attributes: принесён срез по методам.
type FetchProtocolResult struct {
	ServiceName               string
	VersionNumber             int
	Format                    entities.ProtocolFormat
	Path                      string
	NarrowingSkipped          bool
	AttributeNarrowingSkipped bool
}

type SyncProtocolsInput struct {
	// ManifestPath — путь к манифесту зависимостей в репозитории потребителя.
	ManifestPath string
	// DestinationOverride — директория из явного флага; пусто — берём из манифеста.
	DestinationOverride string
}

// SyncProtocolsResult — итог синхронизации: куда разложены контракты и по каждому
// полученному контракту краткая сводка для отчёта пользователю.
type SyncProtocolsResult struct {
	Destination string
	Protocols   []FetchProtocolResult
}

type CheckCompatibilityInput struct {
	ServiceID string
	// Format — формат кандидата (OpenAPI или gRPC, CLI-21); Document читается из
	// CandidatePath — файла контракта-кандидата на диске владельца.
	Format        entities.ProtocolFormat
	CandidatePath string
}

type PublishProtocolInput struct {
	// VersionID — версия, под которой публикуется протокол. Приходит аргументом, а
	// не из манифеста: версия эфемерна, привязана к конкретной выкатке.
	VersionID string
	// ManifestPath — манифест репозитория-владельца; из него берём имя текущего
	// сервиса и путь к его собственному контракту.
	ManifestPath string
}

type PublishVersionInput struct {
	// CommitRevision — развёрнутая ревизия коммита, по которой фиксируется версия.
	CommitRevision string
	// Environment — окружение публикации (DEP-08); дефолт prod задаёт команда.
	Environment string
	// Branch — ветка сборки (DEP-17). Её сообщает пайплайн: CLI сам git не
	// читает — в CI рабочая копия бывает detached, и «текущая ветка» там врёт.
	Branch string
	// ManifestPath — манифест, из которого берём имя текущего сервиса.
	ManifestPath string
	// FormPath — paas.toml с формой сервиса (DEP-02); отсутствие файла — штатно,
	// версия публикуется без формы. Image обязателен вместе с формой.
	FormPath string
	Image    string
}

type RegisterDependencyInput struct {
	// VersionID — версия потребителя, для которой регистрируется состав зависимостей.
	VersionID string
	// ManifestPath — манифест потребителя: из него берём имя своего сервиса и весь
	// состав зависимостей (продьюсеры по имени), а снимки — из его раскладки контрактов.
	ManifestPath string
	// SupersedePrevious — при регистрации каждой зависимости заместить ею зависимости
	// прошлых версий этого потребителя от того же продьюсера (оставить актуальную).
	SupersedePrevious bool
}

// RegisterDependenciesResult — итог массовой регистрации: по каждой зарегистрированной
// зависимости — продьюсер, чтобы отчитаться пользователю.
type RegisterDependenciesResult struct {
	Registered []RegisteredDependency
}

type RegisteredDependency struct {
	ProducerName      string
	ProducerServiceID string
}

type LoginInput struct {
	Email    string
	Password string
}

// LoginResult — итог входа для отчёта пользователю: под кем выполнен вход.
type LoginResult struct {
	Email string
}

// BrowserLoginResult — итог браузерного входа: под кем вошли и до какого числа
// действует выданный токен.
type BrowserLoginResult struct {
	Email     string
	ExpiresAt time.Time
}

// WhoAmIResult — кому принадлежит текущий сохранённый вход. У входа личным
// токеном (AUTH-22) кроме владельца известно, каким именно токеном вошли и
// докуда он действует: платформа e-mail не знает, его помнит сам вход.
type WhoAmIResult struct {
	Email     string
	TokenName string
	ExpiresAt time.Time
}

// LogoutResult — итог выхода. WasLoggedIn=false — сохранённого входа и не было:
// выходить не из чего, но это не ошибка.
type LogoutResult struct {
	WasLoggedIn bool
}

// PublishBuildInput — публикация сборки ветки (DEP-18). Окружения нет: его
// выбирает выкатка.
type PublishBuildInput struct {
	CommitRevision string
	Branch         string
	ManifestPath   string
	FormPath       string
	Image          string
}
