package usecases

type FetchProtocolInput struct {
	ServiceID string
	// Destination — базовая директория для контрактов; конкретный файл внутри
	// формирует хранилище из имени сервиса.
	Destination string
}

// FetchProtocolResult — итог получения протокола для отчёта пользователю.
type FetchProtocolResult struct {
	ServiceName   string
	VersionNumber int
	Path          string
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
