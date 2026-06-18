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

func serviceManifest(contract string) *entities.Manifest {
	return &entities.Manifest{Service: &entities.ManifestService{Name: "paas-backend", Contract: contract}}
}

func TestPublishProtocolExecute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	reader := NewMockCandidateReader(ctrl)
	publisher := NewMockProtocolPublisher(ctrl)

	publication := &entities.ProtocolPublication{VersionNumber: 7, Breaking: true}
	manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(serviceManifest("openapi.json"), nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
	reader.EXPECT().Read(gomock.Any(), "openapi.json").Return([]byte(validDoc), nil)
	publisher.EXPECT().PublishProtocol(gomock.Any(), "svc", "ver", []byte(validDoc)).Return(publication, nil)

	got, err := NewPublishProtocol(manifests, resolver, reader, publisher).Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "protocols.toml"})

	require.NoError(t, err)
	assert.Same(t, publication, got)
}

// Контракт ищется рядом с манифестом, а не относительно текущего каталога.
func TestPublishProtocolExecute_ContractRelativeToManifest(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	reader := NewMockCandidateReader(ctrl)
	publisher := NewMockProtocolPublisher(ctrl)

	manifests.EXPECT().Read(gomock.Any(), "repo/protocols.toml").Return(serviceManifest("api/openapi.json"), nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
	reader.EXPECT().Read(gomock.Any(), "repo/api/openapi.json").Return([]byte(validDoc), nil)
	publisher.EXPECT().PublishProtocol(gomock.Any(), "svc", "ver", []byte(validDoc)).Return(&entities.ProtocolPublication{}, nil)

	_, err := NewPublishProtocol(manifests, resolver, reader, publisher).Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "repo/protocols.toml"})

	require.NoError(t, err)
}

func TestPublishProtocolExecute_NoServiceDeclared_NoResolve(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	reader := NewMockCandidateReader(ctrl)
	publisher := NewMockProtocolPublisher(ctrl)

	// Манифест без секции [service]; платформу и контракт не трогаем.
	manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(&entities.Manifest{}, nil)

	_, err := NewPublishProtocol(manifests, resolver, reader, publisher).Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "protocols.toml"})

	assert.ErrorIs(t, err, entities.ErrManifestNoService)
}

func TestPublishProtocolExecute_ServiceNotFound_NoPublish(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	reader := NewMockCandidateReader(ctrl)
	publisher := NewMockProtocolPublisher(ctrl)

	manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(serviceManifest("openapi.json"), nil)
	// Платформа не знает сервиса — в карту он не попал; контракт не читаем.
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{}, nil)

	_, err := NewPublishProtocol(manifests, resolver, reader, publisher).Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "protocols.toml"})

	assert.ErrorIs(t, err, entities.ErrServiceNotFound)
}

func TestPublishProtocolExecute_ReadError_NoPublish(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	reader := NewMockCandidateReader(ctrl)
	publisher := NewMockProtocolPublisher(ctrl)

	readErr := errors.New("no such file")
	manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(serviceManifest("missing.json"), nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
	reader.EXPECT().Read(gomock.Any(), "missing.json").Return(nil, readErr)
	// PublishProtocol не вызывается — без документа платформу не дёргаем.

	_, err := NewPublishProtocol(manifests, resolver, reader, publisher).Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "protocols.toml"})

	assert.ErrorIs(t, err, readErr)
}

func TestPublishProtocolExecute_InvalidContract_NoPublish(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	reader := NewMockCandidateReader(ctrl)
	publisher := NewMockProtocolPublisher(ctrl)

	manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(serviceManifest("bad.json"), nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
	reader.EXPECT().Read(gomock.Any(), "bad.json").Return([]byte("<html>"), nil)
	// невалидный контракт на платформу не уходит.

	_, err := NewPublishProtocol(manifests, resolver, reader, publisher).Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "protocols.toml"})

	assert.ErrorIs(t, err, entities.ErrInvalidProtocol)
}

func TestPublishProtocolExecute_PublisherError(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	reader := NewMockCandidateReader(ctrl)
	publisher := NewMockProtocolPublisher(ctrl)

	publishErr := errors.New("платформа отклонила публикацию: version not found")
	manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(serviceManifest("openapi.json"), nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
	reader.EXPECT().Read(gomock.Any(), "openapi.json").Return([]byte(validDoc), nil)
	publisher.EXPECT().PublishProtocol(gomock.Any(), "svc", "ver", []byte(validDoc)).Return(nil, publishErr)

	_, err := NewPublishProtocol(manifests, resolver, reader, publisher).Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "protocols.toml"})

	assert.ErrorIs(t, err, publishErr)
}
