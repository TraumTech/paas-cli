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

func TestPublishVersionExecute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := NewMockVersionPublisher(ctrl)

	version := &entities.Version{ID: "ver-1", Number: 7, CommitRevision: "abc123"}
	publisher.EXPECT().PublishVersion(gomock.Any(), "svc", "abc123").Return(version, nil)

	got, err := NewPublishVersion(publisher).Execute(context.Background(),
		PublishVersionInput{ServiceID: "svc", CommitRevision: "abc123"})

	require.NoError(t, err)
	assert.Same(t, version, got)
}

func TestPublishVersionExecute_EmptyRevision_NoPublish(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := NewMockVersionPublisher(ctrl)
	// PublishVersion не вызывается — платформу не дёргаем без ревизии.

	_, err := NewPublishVersion(publisher).Execute(context.Background(),
		PublishVersionInput{ServiceID: "svc", CommitRevision: "  "})

	assert.ErrorIs(t, err, entities.ErrEmptyCommitRevision)
}

func TestPublishVersionExecute_SourceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := NewMockVersionPublisher(ctrl)

	publisher.EXPECT().PublishVersion(gomock.Any(), "svc", "abc123").Return(nil, entities.ErrServiceNotFound)

	_, err := NewPublishVersion(publisher).Execute(context.Background(),
		PublishVersionInput{ServiceID: "svc", CommitRevision: "abc123"})

	assert.ErrorIs(t, err, entities.ErrServiceNotFound)
}

func TestPublishVersionExecute_PropagatesUnknownError(t *testing.T) {
	ctrl := gomock.NewController(t)
	publisher := NewMockVersionPublisher(ctrl)

	srcErr := errors.New("boom")
	publisher.EXPECT().PublishVersion(gomock.Any(), "svc", "abc123").Return(nil, srcErr)

	_, err := NewPublishVersion(publisher).Execute(context.Background(),
		PublishVersionInput{ServiceID: "svc", CommitRevision: "abc123"})

	assert.ErrorIs(t, err, srcErr)
}
