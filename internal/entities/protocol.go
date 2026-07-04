package entities

import (
	"bytes"
	"encoding/json"
)

// ProtocolFormat — формат контракта каноническим именем платформы. Пустое
// значение в манифесте означает формат по умолчанию (OpenAPI).
type ProtocolFormat string

const (
	ProtocolFormatOpenAPI ProtocolFormat = "openapi"
	ProtocolFormatGRPC    ProtocolFormat = "grpc"
)

// ParseProtocolFormat разбирает формат из манифеста. Пусто — формат не указан —
// означает OpenAPI: манифесты, не знающие про тип, работают как прежде.
// Неизвестное имя — понятная ошибка вместо молчаливой публикации не тем типом.
// Здесь — единственная точка CLI, знающая перечень поддерживаемых форматов.
func ParseProtocolFormat(name string) (ProtocolFormat, error) {
	switch name {
	case "", string(ProtocolFormatOpenAPI):
		return ProtocolFormatOpenAPI, nil
	case string(ProtocolFormatGRPC):
		return ProtocolFormatGRPC, nil
	default:
		return "", &UnsupportedProtocolFormatError{Name: name}
	}
}

// Protocol — актуальный опубликованный контракт сервиса: машиночитаемое описание
// его API. Document — сырой документ контракта в родном для формата виде
// (JSON-объект у OpenAPI, .proto-исходник у gRPC), который потребитель кладёт к
// себе и строит против него свой код.
type Protocol struct {
	ServiceID     string
	ServiceName   string
	VersionNumber int
	Format        ProtocolFormat
	Document      []byte
}

// Validate — страховка от затирания рабочего контракта пустым/битым ответом:
// OpenAPI должен быть похож на контракт, у gRPC достаточно непустого текста —
// глубокий разбор .proto остаётся заботой платформы.
func (p *Protocol) Validate() error {
	if p.Format == ProtocolFormatGRPC {
		if len(bytes.TrimSpace(p.Document)) == 0 {
			return ErrEmptyProtocol
		}
		return nil
	}
	return validateContractDocument(p.Document)
}

func validateContractDocument(document []byte) error {
	if len(bytes.TrimSpace(document)) == 0 {
		return ErrEmptyProtocol
	}
	var doc struct {
		OpenAPI string          `json:"openapi"`
		Paths   json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(document, &doc); err != nil {
		return ErrInvalidProtocol
	}
	if doc.OpenAPI == "" || len(doc.Paths) == 0 {
		return ErrInvalidProtocol
	}
	return nil
}
