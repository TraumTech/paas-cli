package entities_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/TraumTech/paas-cli/internal/entities"
)

func TestCandidateContractValidate(t *testing.T) {
	tests := []struct {
		name     string
		document string
		wantErr  error
	}{
		{name: "валидный OpenAPI", document: `{"openapi":"3.1.0","paths":{"/x":{}}}`, wantErr: nil},
		{name: "пустой документ", document: "", wantErr: entities.ErrEmptyProtocol},
		{name: "не JSON", document: "<html>", wantErr: entities.ErrInvalidProtocol},
		{name: "нет paths", document: `{"openapi":"3.1.0"}`, wantErr: entities.ErrInvalidProtocol},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &entities.CandidateContract{Document: []byte(tt.document)}
			assert.ErrorIs(t, c.Validate(), tt.wantErr)
		})
	}
}
