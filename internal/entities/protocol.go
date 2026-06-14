package entities

import (
	"bytes"
	"encoding/json"
)

// Protocol — актуальный опубликованный контракт сервиса: машиночитаемое описание
// его API. Document — сырой документ контракта (для OpenAPI это JSON-объект),
// который потребитель кладёт к себе и строит против него свой код.
type Protocol struct {
	ServiceID     string
	ServiceName   string
	VersionNumber int
	Format        string
	Document      []byte
}

// Validate проверяет, что документ действительно похож на OpenAPI-контракт, а не
// на пустой/битый ответ. Эта проверка — страховка критерия приёмки: рабочий
// контракт не должен затираться чем попало.
func (p *Protocol) Validate() error {
	if len(bytes.TrimSpace(p.Document)) == 0 {
		return ErrEmptyProtocol
	}
	var doc struct {
		OpenAPI string          `json:"openapi"`
		Paths   json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(p.Document, &doc); err != nil {
		return ErrInvalidProtocol
	}
	if doc.OpenAPI == "" || len(doc.Paths) == 0 {
		return ErrInvalidProtocol
	}
	return nil
}
