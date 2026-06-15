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

func TestPublishProtocolExecute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	publisher := NewMockProtocolPublisher(ctrl)

	publication := &entities.ProtocolPublication{VersionNumber: 7, Breaking: true}
	reader.EXPECT().Read(gomock.Any(), "openapi.json").Return([]byte(validDoc), nil)
	publisher.EXPECT().PublishProtocol(gomock.Any(), "svc", "ver", []byte(validDoc)).Return(publication, nil)

	got, err := NewPublishProtocol(reader, publisher).Execute(context.Background(),
		PublishProtocolInput{ServiceID: "svc", VersionID: "ver", ContractPath: "openapi.json"})

	require.NoError(t, err)
	assert.Same(t, publication, got)
}

func TestPublishProtocolExecute_ReadError_NoPublish(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	publisher := NewMockProtocolPublisher(ctrl)

	readErr := errors.New("no such file")
	reader.EXPECT().Read(gomock.Any(), "missing.json").Return(nil, readErr)
	// PublishProtocol не вызывается — без документа платформу не дёргаем.

	_, err := NewPublishProtocol(reader, publisher).Execute(context.Background(),
		PublishProtocolInput{ServiceID: "svc", VersionID: "ver", ContractPath: "missing.json"})

	assert.ErrorIs(t, err, readErr)
}

func TestPublishProtocolExecute_InvalidContract_NoPublish(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	publisher := NewMockProtocolPublisher(ctrl)

	reader.EXPECT().Read(gomock.Any(), "bad.json").Return([]byte("<html>"), nil)
	// невалидный контракт на платформу не уходит.

	_, err := NewPublishProtocol(reader, publisher).Execute(context.Background(),
		PublishProtocolInput{ServiceID: "svc", VersionID: "ver", ContractPath: "bad.json"})

	assert.ErrorIs(t, err, entities.ErrInvalidProtocol)
}

func TestPublishProtocolExecute_PublisherError(t *testing.T) {
	ctrl := gomock.NewController(t)
	reader := NewMockCandidateReader(ctrl)
	publisher := NewMockProtocolPublisher(ctrl)

	publishErr := errors.New("платформа отклонила публикацию: version not found")
	reader.EXPECT().Read(gomock.Any(), "openapi.json").Return([]byte(validDoc), nil)
	publisher.EXPECT().PublishProtocol(gomock.Any(), "svc", "ver", []byte(validDoc)).Return(nil, publishErr)

	_, err := NewPublishProtocol(reader, publisher).Execute(context.Background(),
		PublishProtocolInput{ServiceID: "svc", VersionID: "ver", ContractPath: "openapi.json"})

	assert.ErrorIs(t, err, publishErr)
}
