package entities_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/TraumTech/paas-cli/internal/entities"
)

func TestProtocolValidate(t *testing.T) {
	tests := []struct {
		name     string
		document string
		wantErr  error
	}{
		{name: "валидный OpenAPI", document: `{"openapi":"3.1.0","paths":{"/x":{}}}`, wantErr: nil},
		{name: "пустой документ", document: "", wantErr: entities.ErrEmptyProtocol},
		{name: "пробелы", document: "   \n", wantErr: entities.ErrEmptyProtocol},
		{name: "не JSON", document: "<html>", wantErr: entities.ErrInvalidProtocol},
		{name: "нет openapi", document: `{"paths":{"/x":{}}}`, wantErr: entities.ErrInvalidProtocol},
		{name: "нет paths", document: `{"openapi":"3.1.0"}`, wantErr: entities.ErrInvalidProtocol},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &entities.Protocol{Document: []byte(tt.document)}
			assert.ErrorIs(t, c.Validate(), tt.wantErr)
		})
	}
}

func TestParseProtocolFormat(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want entities.ProtocolFormat
	}{
		{name: "пусто — OpenAPI по умолчанию", in: "", want: entities.ProtocolFormatOpenAPI},
		{name: "openapi", in: "openapi", want: entities.ProtocolFormatOpenAPI},
		{name: "grpc", in: "grpc", want: entities.ProtocolFormatGRPC},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := entities.ParseProtocolFormat(tt.in)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Неизвестный формат — понятная ошибка с именем виновника и перечнем поддерживаемых.
func TestParseProtocolFormat_Unsupported(t *testing.T) {
	_, err := entities.ParseProtocolFormat("graphql")

	var unsupported *entities.UnsupportedProtocolFormatError
	assert.ErrorAs(t, err, &unsupported)
	assert.Contains(t, err.Error(), "graphql")
	assert.Contains(t, err.Error(), "openapi")
	assert.Contains(t, err.Error(), "grpc")
}
