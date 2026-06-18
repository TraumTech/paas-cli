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

func nameManifest() *entities.Manifest {
	return &entities.Manifest{Service: &entities.ManifestService{Name: "paas-backend"}}
}

func TestPublishVersionExecute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	publisher := NewMockVersionPublisher(ctrl)

	version := &entities.Version{ID: "ver-1", Number: 7, CommitRevision: "abc123"}
	manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(nameManifest(), nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
	publisher.EXPECT().PublishVersion(gomock.Any(), "svc", "abc123").Return(version, nil)

	got, err := NewPublishVersion(manifests, resolver, publisher).Execute(context.Background(),
		PublishVersionInput{CommitRevision: "abc123", ManifestPath: "protocols.toml"})

	require.NoError(t, err)
	assert.Same(t, version, got)
}

func TestPublishVersionExecute_EmptyRevision_NoManifest(t *testing.T) {
	ctrl := gomock.NewController(t)
	// Манифест/резолвер/публикацию не трогаем — без ревизии останавливаемся сразу.
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	publisher := NewMockVersionPublisher(ctrl)

	_, err := NewPublishVersion(manifests, resolver, publisher).Execute(context.Background(),
		PublishVersionInput{CommitRevision: "  ", ManifestPath: "protocols.toml"})

	assert.ErrorIs(t, err, entities.ErrEmptyCommitRevision)
}

func TestPublishVersionExecute_NoServiceDeclared_NoPublish(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	publisher := NewMockVersionPublisher(ctrl)

	manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(&entities.Manifest{}, nil)
	// резолвер/публикация не вызываются — манифест не объявляет сервис.

	_, err := NewPublishVersion(manifests, resolver, publisher).Execute(context.Background(),
		PublishVersionInput{CommitRevision: "abc123", ManifestPath: "protocols.toml"})

	assert.ErrorIs(t, err, entities.ErrManifestNoService)
}

func TestPublishVersionExecute_ServiceNotFound_NoPublish(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	publisher := NewMockVersionPublisher(ctrl)

	manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(nameManifest(), nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{}, nil)

	_, err := NewPublishVersion(manifests, resolver, publisher).Execute(context.Background(),
		PublishVersionInput{CommitRevision: "abc123", ManifestPath: "protocols.toml"})

	assert.ErrorIs(t, err, entities.ErrServiceNotFound)
}

func TestPublishVersionExecute_SourceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	publisher := NewMockVersionPublisher(ctrl)

	srcErr := errors.New("boom")
	manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(nameManifest(), nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
	publisher.EXPECT().PublishVersion(gomock.Any(), "svc", "abc123").Return(nil, srcErr)

	_, err := NewPublishVersion(manifests, resolver, publisher).Execute(context.Background(),
		PublishVersionInput{CommitRevision: "abc123", ManifestPath: "protocols.toml"})

	assert.ErrorIs(t, err, srcErr)
}
