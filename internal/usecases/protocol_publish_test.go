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

type publishMocks struct {
	manifests *MockManifestReader
	resolver  *MockServiceResolver
	reader    *MockCandidateReader
	publisher *MockProtocolPublisher
	registry  *MockRegistryDirectory
}

func newPublishMocks(t *testing.T) publishMocks {
	ctrl := gomock.NewController(t)
	return publishMocks{
		manifests: NewMockManifestReader(ctrl),
		resolver:  NewMockServiceResolver(ctrl),
		reader:    NewMockCandidateReader(ctrl),
		publisher: NewMockProtocolPublisher(ctrl),
		registry:  NewMockRegistryDirectory(ctrl),
	}
}

func (m publishMocks) useCase() *PublishProtocolUseCase {
	return NewPublishProtocol(m.manifests, m.resolver, m.reader, m.publisher, m.registry)
}

func TestPublishProtocolExecute_Success(t *testing.T) {
	m := newPublishMocks(t)

	publication := &entities.ProtocolPublication{VersionNumber: 7, Breaking: true}
	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(serviceManifest("openapi.json"), nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
	m.reader.EXPECT().Read(gomock.Any(), "openapi.json").Return([]byte(validDoc), nil)
	m.registry.EXPECT().ListProtocols(gomock.Any(), "svc").Return(nil, nil)
	m.publisher.EXPECT().PublishProtocol(gomock.Any(), "svc", "ver", "", entities.ProtocolFormatOpenAPI, []byte(validDoc)).Return(publication, nil)

	got, err := m.useCase().Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "protocols.toml"})

	require.NoError(t, err)
	require.Len(t, got.Publications, 1)
	assert.Equal(t, *publication, got.Publications[0])
	assert.Empty(t, got.Orphaned)
}

// Каждая запись [[protocols]] публикуется под своим именем и своим форматом
// (CLI-23); итог — публикации в порядке перечня.
func TestPublishProtocolExecute_MultipleProtocols(t *testing.T) {
	m := newPublishMocks(t)

	proto := []byte("syntax = \"proto3\";")
	manifest := &entities.Manifest{
		Service: &entities.ManifestService{Name: "paas-backend"},
		Protocols: []entities.ManifestProtocol{
			{Name: "http", Contract: "openapi.json"},
			{Name: "internal-grpc", Contract: "edge.proto", Format: "grpc"},
		},
	}
	m.manifests.EXPECT().Read(gomock.Any(), "paas.toml").Return(manifest, nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
	m.reader.EXPECT().Read(gomock.Any(), "openapi.json").Return([]byte(validDoc), nil)
	m.reader.EXPECT().Read(gomock.Any(), "edge.proto").Return(proto, nil)
	m.registry.EXPECT().ListProtocols(gomock.Any(), "svc").Return([]entities.RegistryProtocol{
		{Name: "http", Format: "openapi"},
	}, nil)
	m.publisher.EXPECT().PublishProtocol(gomock.Any(), "svc", "ver", "http", entities.ProtocolFormatOpenAPI, []byte(validDoc)).
		Return(&entities.ProtocolPublication{VersionNumber: 3}, nil)
	m.publisher.EXPECT().PublishProtocol(gomock.Any(), "svc", "ver", "internal-grpc", entities.ProtocolFormatGRPC, proto).
		Return(&entities.ProtocolPublication{VersionNumber: 3}, nil)

	got, err := m.useCase().Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "paas.toml"})

	require.NoError(t, err)
	require.Len(t, got.Publications, 2)
	assert.Equal(t, "http", got.Publications[0].Name)
	assert.Equal(t, "internal-grpc", got.Publications[1].Name)
}

// Протокол по умолчанию исчез из манифеста (например, имя переименовали), а от
// него зависят потребители — публикация удерживается целиком: для них это
// ломающее изменение (PRT-09).
func TestPublishProtocolExecute_OrphanedWithConsumers_Held(t *testing.T) {
	m := newPublishMocks(t)

	manifest := &entities.Manifest{
		Service:   &entities.ManifestService{Name: "paas-backend"},
		Protocols: []entities.ManifestProtocol{{Name: "http", Contract: "openapi.json"}},
	}
	m.manifests.EXPECT().Read(gomock.Any(), "paas.toml").Return(manifest, nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
	m.reader.EXPECT().Read(gomock.Any(), "openapi.json").Return([]byte(validDoc), nil)
	m.registry.EXPECT().ListProtocols(gomock.Any(), "svc").Return([]entities.RegistryProtocol{
		{Name: entities.DefaultProtocolName, Format: "openapi"},
	}, nil)
	m.registry.EXPECT().ListConsumers(gomock.Any(), "svc").Return([]entities.RegisteredConsumer{
		{ServiceName: "paas-frontend", VersionNumber: 12},
	}, nil)
	// публикаций нет — гейт держит перечень целиком.

	_, err := m.useCase().Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "paas.toml"})

	var orphaned *entities.OrphanedProtocolError
	require.ErrorAs(t, err, &orphaned)
	assert.Equal(t, entities.DefaultProtocolName, orphaned.Name)
	assert.ErrorContains(t, err, "paas-frontend v12")
}

// Исчезнувший протокол без потребителей публикацию не держит, но попадает в
// предупреждения отчёта. Потребители спрашиваются только у протокола по
// умолчанию: до «зависимости от именованного протокола» других потребителей
// не бывает.
func TestPublishProtocolExecute_OrphanedWithoutConsumers_Warns(t *testing.T) {
	m := newPublishMocks(t)

	manifest := &entities.Manifest{
		Service:   &entities.ManifestService{Name: "paas-backend"},
		Protocols: []entities.ManifestProtocol{{Name: "http", Contract: "openapi.json"}},
	}
	m.manifests.EXPECT().Read(gomock.Any(), "paas.toml").Return(manifest, nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
	m.reader.EXPECT().Read(gomock.Any(), "openapi.json").Return([]byte(validDoc), nil)
	m.registry.EXPECT().ListProtocols(gomock.Any(), "svc").Return([]entities.RegistryProtocol{
		{Name: "http", Format: "openapi"},
		{Name: "admin", Format: "openapi"},
	}, nil)
	m.publisher.EXPECT().PublishProtocol(gomock.Any(), "svc", "ver", "http", entities.ProtocolFormatOpenAPI, []byte(validDoc)).
		Return(&entities.ProtocolPublication{VersionNumber: 4}, nil)

	got, err := m.useCase().Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "paas.toml"})

	require.NoError(t, err)
	assert.Equal(t, []string{"admin"}, got.Orphaned)
}

// Изъян любой записи (здесь — битый контракт второй) обнаруживается до первой
// публикации: перечень не публикуется наполовину.
func TestPublishProtocolExecute_InvalidSecondContract_NothingPublished(t *testing.T) {
	m := newPublishMocks(t)

	manifest := &entities.Manifest{
		Service: &entities.ManifestService{Name: "paas-backend"},
		Protocols: []entities.ManifestProtocol{
			{Name: "http", Contract: "openapi.json"},
			{Name: "admin", Contract: "admin.json"},
		},
	}
	m.manifests.EXPECT().Read(gomock.Any(), "paas.toml").Return(manifest, nil)
	m.reader.EXPECT().Read(gomock.Any(), "openapi.json").Return([]byte(validDoc), nil)
	m.reader.EXPECT().Read(gomock.Any(), "admin.json").Return([]byte("<html>"), nil)
	// ни резолва, ни публикаций — контракты проверяются до похода на платформу.

	_, err := m.useCase().Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "paas.toml"})

	assert.ErrorIs(t, err, entities.ErrInvalidProtocol)
	assert.ErrorContains(t, err, `протокол "admin"`)
}

// Контракт ищется рядом с манифестом, а не относительно текущего каталога.
func TestPublishProtocolExecute_ContractRelativeToManifest(t *testing.T) {
	m := newPublishMocks(t)

	m.manifests.EXPECT().Read(gomock.Any(), "repo/protocols.toml").Return(serviceManifest("api/openapi.json"), nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
	m.reader.EXPECT().Read(gomock.Any(), "repo/api/openapi.json").Return([]byte(validDoc), nil)
	m.registry.EXPECT().ListProtocols(gomock.Any(), "svc").Return(nil, nil)
	m.publisher.EXPECT().PublishProtocol(gomock.Any(), "svc", "ver", "", entities.ProtocolFormatOpenAPI, []byte(validDoc)).Return(&entities.ProtocolPublication{}, nil)

	_, err := m.useCase().Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "repo/protocols.toml"})

	require.NoError(t, err)
}

// gRPC-контракт из манифеста уходит на платформу своим форматом; JSON-проверка
// OpenAPI к .proto-исходнику не применяется.
func TestPublishProtocolExecute_GRPCFormat(t *testing.T) {
	m := newPublishMocks(t)

	proto := []byte("syntax = \"proto3\";\npackage traumtech.paas_protocols.v1;")
	publication := &entities.ProtocolPublication{VersionNumber: 1}
	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(grpcServiceManifest("registry.proto"), nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-protocols"}).Return(map[string]string{"paas-protocols": "svc"}, nil)
	m.reader.EXPECT().Read(gomock.Any(), "registry.proto").Return(proto, nil)
	m.registry.EXPECT().ListProtocols(gomock.Any(), "svc").Return(nil, nil)
	m.publisher.EXPECT().PublishProtocol(gomock.Any(), "svc", "ver", "", entities.ProtocolFormatGRPC, proto).Return(publication, nil)

	got, err := m.useCase().Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "protocols.toml"})

	require.NoError(t, err)
	require.Len(t, got.Publications, 1)
	assert.Equal(t, *publication, got.Publications[0])
}

// Опечатка в формате — ошибка до похода на платформу: ни резолва, ни чтения
// контракта, ни публикации не тем типом.
func TestPublishProtocolExecute_UnsupportedFormat_NoPublish(t *testing.T) {
	m := newPublishMocks(t)

	manifest := &entities.Manifest{Service: &entities.ManifestService{Name: "svc", Contract: "c.json", Format: "graphql"}}
	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(manifest, nil)

	_, err := m.useCase().Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "protocols.toml"})

	var unsupported *entities.UnsupportedProtocolFormatError
	require.ErrorAs(t, err, &unsupported)
	assert.Equal(t, "graphql", unsupported.Name)
}

func TestPublishProtocolExecute_EmptyGRPCContract_NoPublish(t *testing.T) {
	m := newPublishMocks(t)

	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(grpcServiceManifest("registry.proto"), nil)
	m.reader.EXPECT().Read(gomock.Any(), "registry.proto").Return([]byte("  \n"), nil)
	// пустой контракт на платформу не уходит: ни резолва, ни публикации.

	_, err := m.useCase().Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "protocols.toml"})

	assert.ErrorIs(t, err, entities.ErrEmptyProtocol)
}

func TestPublishProtocolExecute_NoServiceDeclared_NoResolve(t *testing.T) {
	m := newPublishMocks(t)

	// Манифест без секции [service]; платформу и контракт не трогаем.
	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(&entities.Manifest{}, nil)

	_, err := m.useCase().Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "protocols.toml"})

	assert.ErrorIs(t, err, entities.ErrManifestNoService)
}

func TestPublishProtocolExecute_ServiceNotFound_NoPublish(t *testing.T) {
	m := newPublishMocks(t)

	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(serviceManifest("openapi.json"), nil)
	m.reader.EXPECT().Read(gomock.Any(), "openapi.json").Return([]byte(validDoc), nil)
	// Платформа не знает сервиса — в карту он не попал; публикации нет.
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{}, nil)

	_, err := m.useCase().Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "protocols.toml"})

	assert.ErrorIs(t, err, entities.ErrServiceNotFound)
}

func TestPublishProtocolExecute_ReadError_NoPublish(t *testing.T) {
	m := newPublishMocks(t)

	readErr := errors.New("no such file")
	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(serviceManifest("missing.json"), nil)
	m.reader.EXPECT().Read(gomock.Any(), "missing.json").Return(nil, readErr)
	// PublishProtocol не вызывается — без документа платформу не дёргаем.

	_, err := m.useCase().Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "protocols.toml"})

	assert.ErrorIs(t, err, readErr)
}

func TestPublishProtocolExecute_InvalidContract_NoPublish(t *testing.T) {
	m := newPublishMocks(t)

	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(serviceManifest("bad.json"), nil)
	m.reader.EXPECT().Read(gomock.Any(), "bad.json").Return([]byte("<html>"), nil)
	// невалидный контракт на платформу не уходит.

	_, err := m.useCase().Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "protocols.toml"})

	assert.ErrorIs(t, err, entities.ErrInvalidProtocol)
}

func TestPublishProtocolExecute_PublisherError(t *testing.T) {
	m := newPublishMocks(t)

	publishErr := errors.New("платформа отклонила публикацию: version not found")
	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(serviceManifest("openapi.json"), nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
	m.reader.EXPECT().Read(gomock.Any(), "openapi.json").Return([]byte(validDoc), nil)
	m.registry.EXPECT().ListProtocols(gomock.Any(), "svc").Return(nil, nil)
	m.publisher.EXPECT().PublishProtocol(gomock.Any(), "svc", "ver", "", entities.ProtocolFormatOpenAPI, []byte(validDoc)).Return(nil, publishErr)

	_, err := m.useCase().Execute(context.Background(),
		PublishProtocolInput{VersionID: "ver", ManifestPath: "protocols.toml"})

	assert.ErrorIs(t, err, publishErr)
}
