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

func grpcServiceManifest(contract string) *entities.Manifest {
	return &entities.Manifest{Service: &entities.ManifestService{Name: "paas-protocols", Contract: contract, Format: "grpc"}}
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
	publisher.EXPECT().PublishProtocol(gomock.Any(), "svc", "ver", entities.ProtocolFormatOpenAPI, []byte(validDoc)).Return(publication, nil)

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
	publisher.EXPECT().PublishProtocol(gomock.Any(), "svc", "ver", entities.ProtocolFormatOpenAPI, []byte(validDoc)).Return(&entities.ProtocolPublication{}, nil)

	_, err := NewPublishProtocol(manifests, resolver, reader, publisher).Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "repo/protocols.toml"})

	require.NoError(t, err)
}

// gRPC-контракт из манифеста уходит на платформу своим форматом; JSON-проверка
// OpenAPI к .proto-исходнику не применяется.
func TestPublishProtocolExecute_GRPCFormat(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	reader := NewMockCandidateReader(ctrl)
	publisher := NewMockProtocolPublisher(ctrl)

	proto := []byte("syntax = \"proto3\";\npackage traumtech.paas_protocols.v1;")
	publication := &entities.ProtocolPublication{VersionNumber: 1}
	manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(grpcServiceManifest("registry.proto"), nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-protocols"}).Return(map[string]string{"paas-protocols": "svc"}, nil)
	reader.EXPECT().Read(gomock.Any(), "registry.proto").Return(proto, nil)
	publisher.EXPECT().PublishProtocol(gomock.Any(), "svc", "ver", entities.ProtocolFormatGRPC, proto).Return(publication, nil)

	got, err := NewPublishProtocol(manifests, resolver, reader, publisher).Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "protocols.toml"})

	require.NoError(t, err)
	assert.Same(t, publication, got)
}

// Опечатка в формате — ошибка до похода на платформу: ни резолва, ни чтения
// контракта, ни публикации не тем типом.
func TestPublishProtocolExecute_UnsupportedFormat_NoPublish(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	reader := NewMockCandidateReader(ctrl)
	publisher := NewMockProtocolPublisher(ctrl)

	manifest := &entities.Manifest{Service: &entities.ManifestService{Name: "svc", Contract: "c.json", Format: "graphql"}}
	manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(manifest, nil)

	_, err := NewPublishProtocol(manifests, resolver, reader, publisher).Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "protocols.toml"})

	var unsupported *entities.UnsupportedProtocolFormatError
	require.ErrorAs(t, err, &unsupported)
	assert.Equal(t, "graphql", unsupported.Name)
}

func TestPublishProtocolExecute_EmptyGRPCContract_NoPublish(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	reader := NewMockCandidateReader(ctrl)
	publisher := NewMockProtocolPublisher(ctrl)

	manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(grpcServiceManifest("registry.proto"), nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-protocols"}).Return(map[string]string{"paas-protocols": "svc"}, nil)
	reader.EXPECT().Read(gomock.Any(), "registry.proto").Return([]byte("  \n"), nil)
	// пустой контракт на платформу не уходит.

	_, err := NewPublishProtocol(manifests, resolver, reader, publisher).Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "protocols.toml"})

	assert.ErrorIs(t, err, entities.ErrEmptyProtocol)
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
	publisher.EXPECT().PublishProtocol(gomock.Any(), "svc", "ver", entities.ProtocolFormatOpenAPI, []byte(validDoc)).Return(nil, publishErr)

	_, err := NewPublishProtocol(manifests, resolver, reader, publisher).Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "protocols.toml"})

	assert.ErrorIs(t, err, publishErr)
}
