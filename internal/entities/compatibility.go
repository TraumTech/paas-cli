package entities

import "bytes"

// CandidateContract — контракт из локального файла, который владелец сервиса
// отправляет платформе: либо на проверку совместимости (без публикации), либо при
// публикации протокола под версией. Пустой Format означает OpenAPI — как в
// манифесте, где формат необязателен.
type CandidateContract struct {
	Format   ProtocolFormat
	Document []byte
}

// Validate — страховка от отправки заведомо пустого/битого файла: OpenAPI должен
// быть похож на контракт, у gRPC достаточно непустого текста — синтаксис .proto
// проверяет платформа, CLI её разбор не дублирует.
func (c *CandidateContract) Validate() error {
	if c.Format == ProtocolFormatGRPC {
		if len(bytes.TrimSpace(c.Document)) == 0 {
			return ErrEmptyProtocol
		}
		return nil
	}
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
