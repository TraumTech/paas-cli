package entities

// CandidateContract — контракт из локального файла, который владелец сервиса
// отправляет платформе: либо на проверку совместимости (без публикации), либо при
// публикации протокола под версией.
type CandidateContract struct {
	Document []byte
}

func (c *CandidateContract) Validate() error {
	return validateContractDocument(c.Document)
}

// CompatibilityReport — разбор совместимости кандидата со снимками потребителей.
// Breaking — сводный признак: ломает ли кандидат хотя бы одного потребителя.
type CompatibilityReport struct {
	Breaking  bool
	Consumers []ConsumerCompatibility
}

// ConsumerCompatibility — вердикт по одному потребителю. Comparable=false означает,
// что снимок потребителя не разобран для сравнения; такой случай ломающим не
// считается.
type ConsumerCompatibility struct {
	ServiceName   string
	VersionNumber int
	Comparable    bool
	Breaking      bool
	Changes       []CompatibilityChange
}

// CompatibilityChange — одно изменение контракта относительно снимка потребителя.
type CompatibilityChange struct {
	Breaking    bool
	Kind        string
	Operation   string
	Description string
}
