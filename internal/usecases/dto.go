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
	ServiceID string
	VersionID string
	// ContractPath — путь к файлу контракта в репозитории владельца сервиса.
	ContractPath string
}

type PublishVersionInput struct {
	ServiceID string
	// CommitRevision — развёрнутая ревизия коммита, по которой фиксируется версия.
	CommitRevision string
}

type RegisterDependencyInput struct {
	// ServiceID — сервис-потребитель (собственный), чья версия зависит от контракта.
	ServiceID string
	// VersionID — версия потребителя, для которой регистрируется зависимость.
	VersionID string
	// ProducerServiceID — сервис-продьюсер, на контракте которого собрана версия.
	ProducerServiceID string
	// ContractPath — путь к файлу контракта продьюсера в репозитории потребителя:
	// снимок, на котором собрана эта версия.
	ContractPath string
}
