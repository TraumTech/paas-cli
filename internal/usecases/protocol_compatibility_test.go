package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// gRPC-кандидат (CLI-21): формат доходит до платформы, .proto не гоняется через
// JSON-проверку OpenAPI; пустой .proto — честный отказ до платформы.
func TestCheckCompatibilityExecute_GRPCCandidate(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	source := NewMockCompatibilitySource(ctrl)

	proto := []byte("syntax = \"proto3\";\npackage traumtech.paas_protocols.v1;")
	report := &entities.CompatibilityReport{Breaking: false}
	reader.EXPECT().Read(gomock.Any(), "registry.proto").Return(proto, nil)
	source.EXPECT().CheckCompatibility(gomock.Any(), "svc", entities.ProtocolFormatGRPC, proto).Return(report, nil)

	got, err := NewCheckCompatibility(reader, source).Execute(context.Background(),
		CheckCompatibilityInput{ServiceID: "svc", Format: entities.ProtocolFormatGRPC, CandidatePath: "registry.proto"})

	require.NoError(t, err)
	assert.Same(t, report, got)
}

func TestCheckCompatibilityExecute_EmptyGRPCCandidate(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	source := NewMockCompatibilitySource(ctrl)

	reader.EXPECT().Read(gomock.Any(), "registry.proto").Return([]byte("  \n"), nil)
	// платформа не вызывается.

	_, err := NewCheckCompatibility(reader, source).Execute(context.Background(),
		CheckCompatibilityInput{ServiceID: "svc", Format: entities.ProtocolFormatGRPC, CandidatePath: "registry.proto"})

	assert.ErrorIs(t, err, entities.ErrEmptyProtocol)
}

func TestCheckCompatibilityExecute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	source := NewMockCompatibilitySource(ctrl)

	report := &entities.CompatibilityReport{Breaking: true}
	reader.EXPECT().Read(gomock.Any(), "openapi.json").Return([]byte(validDoc), nil)
	source.EXPECT().CheckCompatibility(gomock.Any(), "svc", entities.ProtocolFormatOpenAPI, []byte(validDoc)).Return(report, nil)

	got, err := NewCheckCompatibility(reader, source).Execute(context.Background(),
		CheckCompatibilityInput{ServiceID: "svc", Format: entities.ProtocolFormatOpenAPI, CandidatePath: "openapi.json"})

	require.NoError(t, err)
	assert.Same(t, report, got)
}

func TestCheckCompatibilityExecute_ReadError_NoCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	source := NewMockCompatibilitySource(ctrl)

	readErr := errors.New("no such file")
	reader.EXPECT().Read(gomock.Any(), "missing.json").Return(nil, readErr)
	// CheckCompatibility не вызывается — платформу не дёргаем без документа.

	_, err := NewCheckCompatibility(reader, source).Execute(context.Background(),
		CheckCompatibilityInput{ServiceID: "svc", CandidatePath: "missing.json"})

	assert.ErrorIs(t, err, readErr)
}

func TestCheckCompatibilityExecute_InvalidCandidate_NoCheck(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	source := NewMockCompatibilitySource(ctrl)

	reader.EXPECT().Read(gomock.Any(), "bad.json").Return([]byte("<html>"), nil)
	// невалидный кандидат на платформу не уходит.

	_, err := NewCheckCompatibility(reader, source).Execute(context.Background(),
		CheckCompatibilityInput{ServiceID: "svc", CandidatePath: "bad.json"})

	assert.ErrorIs(t, err, entities.ErrInvalidProtocol)
}

func TestCheckCompatibilityExecute_SourceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	source := NewMockCompatibilitySource(ctrl)

	reader.EXPECT().Read(gomock.Any(), "openapi.json").Return([]byte(validDoc), nil)
	source.EXPECT().CheckCompatibility(gomock.Any(), "svc", entities.ProtocolFormatOpenAPI, []byte(validDoc)).Return(nil, entities.ErrServiceNotFound)

	_, err := NewCheckCompatibility(reader, source).Execute(context.Background(),
		CheckCompatibilityInput{ServiceID: "svc", Format: entities.ProtocolFormatOpenAPI, CandidatePath: "openapi.json"})

	assert.ErrorIs(t, err, entities.ErrServiceNotFound)
}
