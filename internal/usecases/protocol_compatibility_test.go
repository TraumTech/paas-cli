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

func TestCheckCompatibilityExecute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	source := NewMockCompatibilitySource(ctrl)

	report := &entities.CompatibilityReport{Breaking: true}
	reader.EXPECT().Read(gomock.Any(), "openapi.json").Return([]byte(validDoc), nil)
	source.EXPECT().CheckCompatibility(gomock.Any(), "svc", []byte(validDoc)).Return(report, nil)

	got, err := NewCheckCompatibility(reader, source).Execute(context.Background(),
		CheckCompatibilityInput{ServiceID: "svc", CandidatePath: "openapi.json"})

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
	source.EXPECT().CheckCompatibility(gomock.Any(), "svc", []byte(validDoc)).Return(nil, entities.ErrServiceNotFound)

	_, err := NewCheckCompatibility(reader, source).Execute(context.Background(),
		CheckCompatibilityInput{ServiceID: "svc", CandidatePath: "openapi.json"})

	assert.ErrorIs(t, err, entities.ErrServiceNotFound)
}
