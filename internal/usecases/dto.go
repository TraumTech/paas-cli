package usecases

type FetchProtocolInput struct {
	ServiceID string
	// Destination — базовая директория для контрактов; конкретный файл внутри
	// формирует хранилище из имени сервиса.
	Destination string
	// Methods — operationId методов, которые нужно оставить в контракте. Пусто —
	// получаем контракт целиком; иначе платформенный контракт обрезается до них.
	Methods []string
}

// FetchProtocolResult — итог получения протокола для отчёта пользователю.
type FetchProtocolResult struct {
	ServiceName   string
	VersionNumber int
	Path          string
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
	// CandidatePath — путь к файлу контракта-кандидата на диске потребителя.
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
	// ManifestPath — манифест, из которого берём имя текущего сервиса.
	ManifestPath string
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
