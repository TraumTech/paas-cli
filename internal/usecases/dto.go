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
