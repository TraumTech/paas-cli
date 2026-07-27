package usecases

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/TraumTech/paas-cli/internal/entities"
)

const validContract = `{"openapi":"3.1.0","paths":{"/x":{}}}`

func consumerManifest(deps ...string) *entities.Manifest {
	m := &entities.Manifest{Service: &entities.ManifestService{Name: "paas-frontend"}}
	for _, d := range deps {
		m.Dependencies = append(m.Dependencies, entities.ManifestDependency{Name: d})
	}
	return m
}

type registerMocks struct {
	manifests *MockManifestReader
	resolver  *MockServiceResolver
	reader    *MockCandidateReader
	registrar *MockDependencyRegistrar
}

func newRegisterMocks(ctrl *gomock.Controller) registerMocks {
	return registerMocks{
		manifests: NewMockManifestReader(ctrl),
		resolver:  NewMockServiceResolver(ctrl),
		reader:    NewMockCandidateReader(ctrl),
		registrar: NewMockDependencyRegistrar(ctrl),
	}
}

func (m registerMocks) run() (*RegisterDependenciesResult, error) {
	return NewRegisterDependency(m.manifests, m.resolver, m.reader, m.registrar).
		Execute(context.Background(), RegisterDependencyInput{VersionID: "ver-1", ManifestPath: "protocols.toml"})
}

func TestRegisterDependencyExecute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newRegisterMocks(ctrl)

	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(consumerManifest("paas-backend"), nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-frontend", "paas-backend"}).
		Return(map[string]string{"paas-frontend": "consumer", "paas-backend": "prod"}, nil)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-backend/openapi.json").Return([]byte(validContract), nil)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-backend/contract.proto").Return(nil, os.ErrNotExist)
	m.registrar.EXPECT().
		RegisterDependency(gomock.Any(), gomock.Any()).
		Return(&entities.Dependency{ConsumerVersionID: "ver-1", ProducerServiceID: "prod"}, nil)

	got, err := m.run()

	require.NoError(t, err)
	require.Len(t, got.Registered, 1)
	assert.Equal(t, "paas-backend", got.Registered[0].ProducerName)
	assert.Equal(t, "prod", got.Registered[0].ProducerServiceID)
}

// Перечень используемых методов из манифеста доходит до регистратора как есть.
func TestRegisterDependencyExecute_PassesMethods(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newRegisterMocks(ctrl)

	manifest := &entities.Manifest{
		Service: &entities.ManifestService{Name: "paas-frontend"},
		Dependencies: []entities.ManifestDependency{
			{Name: "paas-backend", Methods: []string{"GET /services/{id}", "GET /services/{id}/protocol"}},
		},
	}
	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(manifest, nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-frontend", "paas-backend"}).
		Return(map[string]string{"paas-frontend": "consumer", "paas-backend": "prod"}, nil)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-backend/openapi.json").Return([]byte(validContract), nil)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-backend/contract.proto").Return(nil, os.ErrNotExist)
	m.registrar.EXPECT().
		RegisterDependency(gomock.Any(), DependencyRegistration{
			ServiceID: "consumer", VersionID: "ver-1", ProducerServiceID: "prod",
			Format: entities.ProtocolFormatOpenAPI, Document: []byte(validContract),
			Methods: []string{"GET /services/{id}", "GET /services/{id}/protocol"},
		}).
		Return(&entities.Dependency{}, nil)

	_, err := m.run()
	require.NoError(t, err)
}

// PRT-26: отказы от атрибутов из манифеста доходят до регистратора как есть.
func TestRegisterDependencyExecute_PassesWaivedAttributes(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newRegisterMocks(ctrl)

	waived := []string{"traumtech.paas_services.v1.Service.owner_id"}
	manifest := &entities.Manifest{
		Service: &entities.ManifestService{Name: "paas-frontend"},
		Dependencies: []entities.ManifestDependency{
			{Name: "paas-backend", Waived: waived},
		},
	}
	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(manifest, nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-frontend", "paas-backend"}).
		Return(map[string]string{"paas-frontend": "consumer", "paas-backend": "prod"}, nil)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-backend/openapi.json").Return([]byte(validContract), nil)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-backend/contract.proto").Return(nil, os.ErrNotExist)
	m.registrar.EXPECT().
		RegisterDependency(gomock.Any(), DependencyRegistration{
			ServiceID: "consumer", VersionID: "ver-1", ProducerServiceID: "prod",
			Format: entities.ProtocolFormatOpenAPI, Document: []byte(validContract),
			Waived: waived,
		}).Return(&entities.Dependency{}, nil)

	_, err := m.run()
	require.NoError(t, err)
}

// С SupersedePrevious флаг замещения уходит в каждый вызов регистрации.
func TestRegisterDependencyExecute_SupersedePrevious(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newRegisterMocks(ctrl)

	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(consumerManifest("paas-backend"), nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-frontend", "paas-backend"}).
		Return(map[string]string{"paas-frontend": "consumer", "paas-backend": "prod"}, nil)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-backend/openapi.json").Return([]byte(validContract), nil)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-backend/contract.proto").Return(nil, os.ErrNotExist)
	m.registrar.EXPECT().
		RegisterDependency(gomock.Any(), gomock.Any()).
		Return(&entities.Dependency{}, nil)

	_, err := NewRegisterDependency(m.manifests, m.resolver, m.reader, m.registrar).
		Execute(context.Background(), RegisterDependencyInput{VersionID: "ver-1", ManifestPath: "protocols.toml", SupersedePrevious: true})

	require.NoError(t, err)
}

func TestRegisterDependencyExecute_AllDependencies(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newRegisterMocks(ctrl)

	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(consumerManifest("paas-backend", "billing"), nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-frontend", "paas-backend", "billing"}).
		Return(map[string]string{"paas-frontend": "consumer", "paas-backend": "prod-a", "billing": "prod-b"}, nil)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-backend/openapi.json").Return([]byte(validContract), nil)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-backend/contract.proto").Return(nil, os.ErrNotExist)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/billing/openapi.json").Return([]byte(validContract), nil)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/billing/contract.proto").Return(nil, os.ErrNotExist)
	m.registrar.EXPECT().RegisterDependency(gomock.Any(), gomock.Cond(func(in DependencyRegistration) bool {
		return in.ProducerServiceID == "prod-a"
	})).Return(&entities.Dependency{}, nil)
	m.registrar.EXPECT().RegisterDependency(gomock.Any(), gomock.Cond(func(in DependencyRegistration) bool {
		return in.ProducerServiceID == "prod-b"
	})).Return(&entities.Dependency{}, nil)

	got, err := m.run()

	require.NoError(t, err)
	require.Len(t, got.Registered, 2)
	assert.Equal(t, "billing", got.Registered[1].ProducerName)
}

// destination из манифеста меняет, откуда берутся снимки.
func TestRegisterDependencyExecute_DestinationFromManifest(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newRegisterMocks(ctrl)

	manifest := consumerManifest("paas-backend")
	manifest.Destination = "vendor/api"
	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(manifest, nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-frontend", "paas-backend"}).
		Return(map[string]string{"paas-frontend": "consumer", "paas-backend": "prod"}, nil)
	m.reader.EXPECT().Read(gomock.Any(), "vendor/api/paas-backend/openapi.json").Return([]byte(validContract), nil)
	m.reader.EXPECT().Read(gomock.Any(), "vendor/api/paas-backend/contract.proto").Return(nil, os.ErrNotExist)
	m.registrar.EXPECT().RegisterDependency(gomock.Any(), gomock.Any()).Return(&entities.Dependency{}, nil)

	_, err := m.run()
	require.NoError(t, err)
}

func TestRegisterDependencyExecute_NoService_NoRegister(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newRegisterMocks(ctrl)

	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(&entities.Manifest{}, nil)
	// резолвер/чтение/регистрация не вызываются — манифест не объявляет сервис.

	_, err := m.run()
	assert.ErrorIs(t, err, entities.ErrManifestNoService)
}

func TestRegisterDependencyExecute_NoDependencies_NoRegister(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newRegisterMocks(ctrl)

	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(consumerManifest(), nil)

	_, err := m.run()
	assert.ErrorIs(t, err, entities.ErrManifestNoDependencies)
}

func TestRegisterDependencyExecute_ConsumerNotFound_NoRegister(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newRegisterMocks(ctrl)

	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(consumerManifest("paas-backend"), nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-frontend", "paas-backend"}).
		Return(map[string]string{"paas-backend": "prod"}, nil)

	_, err := m.run()
	assert.ErrorIs(t, err, entities.ErrServiceNotFound)
}

func TestRegisterDependencyExecute_ProducerNotFound_Aborts(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newRegisterMocks(ctrl)

	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(consumerManifest("ghost"), nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-frontend", "ghost"}).
		Return(map[string]string{"paas-frontend": "consumer"}, nil)
	// снимок не читаем и не регистрируем — продьюсер не найден.

	_, err := m.run()
	assert.ErrorIs(t, err, entities.ErrServiceNotFound)
	assert.Contains(t, err.Error(), "ghost")
}

func TestRegisterDependencyExecute_SnapshotReadError_Aborts(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newRegisterMocks(ctrl)

	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(consumerManifest("paas-backend"), nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-frontend", "paas-backend"}).
		Return(map[string]string{"paas-frontend": "consumer", "paas-backend": "prod"}, nil)
	readErr := errors.New("no such file")
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-backend/openapi.json").Return(nil, readErr)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-backend/contract.proto").Return(nil, os.ErrNotExist)
	// RegisterDependency не вызывается — снимок не прочитан.

	_, err := m.run()
	assert.ErrorIs(t, err, readErr)
	assert.Contains(t, err.Error(), "paas-backend")
}

func TestRegisterDependencyExecute_InvalidSnapshot_Aborts(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newRegisterMocks(ctrl)

	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(consumerManifest("paas-backend"), nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-frontend", "paas-backend"}).
		Return(map[string]string{"paas-frontend": "consumer", "paas-backend": "prod"}, nil)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-backend/openapi.json").Return([]byte(`{"not":"openapi"}`), nil)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-backend/contract.proto").Return(nil, os.ErrNotExist)

	_, err := m.run()
	assert.ErrorIs(t, err, entities.ErrInvalidProtocol)
}

// gRPC-зависимость (CLI-20): формат определяется по раскладке (contract.proto),
// снимок и методы gRPC-идентичностью уходят в реестр.
func TestRegisterDependencyExecute_GRPCSnapshotFromLayout(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newRegisterMocks(ctrl)

	manifest := consumerManifest("paas-protocols")
	manifest.Dependencies[0].Methods = []string{"traumtech.paas_protocols.v1.RegistryService/PublishProtocol"}
	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(manifest, nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-frontend", "paas-protocols"}).
		Return(map[string]string{"paas-frontend": "consumer", "paas-protocols": "prod"}, nil)
	proto := []byte("syntax = \"proto3\";\npackage traumtech.paas_protocols.v1;")
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-protocols/openapi.json").Return(nil, os.ErrNotExist)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-protocols/contract.proto").Return(proto, nil)
	m.registrar.EXPECT().RegisterDependency(gomock.Any(), DependencyRegistration{
		ServiceID: "consumer", VersionID: "ver-1", ProducerServiceID: "prod",
		Format: entities.ProtocolFormatGRPC, Document: proto,
		Methods: []string{"traumtech.paas_protocols.v1.RegistryService/PublishProtocol"},
	}).Return(&entities.Dependency{}, nil)

	res, err := m.run()
	require.NoError(t, err)
	assert.Len(t, res.Registered, 1)
}

// Оба снимка в раскладке — неоднозначность: честная ошибка, а не молчаливый выбор.
func TestRegisterDependencyExecute_AmbiguousLayout_Aborts(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newRegisterMocks(ctrl)

	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(consumerManifest("paas-backend"), nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), gomock.Any()).
		Return(map[string]string{"paas-frontend": "consumer", "paas-backend": "prod"}, nil)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-backend/openapi.json").Return([]byte(validContract), nil)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-backend/contract.proto").Return([]byte("syntax..."), nil)
	// Регистрация не вызывается.

	_, err := m.run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "два снимка")
	assert.Contains(t, err.Error(), "paas-backend")
}

// Пустой gRPC-снимок в раскладке — понятная ошибка с именем зависимости.
func TestRegisterDependencyExecute_EmptyGRPCSnapshot_Aborts(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newRegisterMocks(ctrl)

	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(consumerManifest("paas-protocols"), nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), gomock.Any()).
		Return(map[string]string{"paas-frontend": "consumer", "paas-protocols": "prod"}, nil)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-protocols/openapi.json").Return(nil, os.ErrNotExist)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-protocols/contract.proto").Return([]byte("  \n"), nil)

	_, err := m.run()
	assert.ErrorIs(t, err, entities.ErrEmptyProtocol)
	assert.Contains(t, err.Error(), "paas-protocols")
}

func TestRegisterDependencyExecute_RegistrarError_Aborts(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newRegisterMocks(ctrl)

	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(consumerManifest("paas-backend"), nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-frontend", "paas-backend"}).
		Return(map[string]string{"paas-frontend": "consumer", "paas-backend": "prod"}, nil)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-backend/openapi.json").Return([]byte(validContract), nil)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-backend/contract.proto").Return(nil, os.ErrNotExist)
	srcErr := errors.New("boom")
	m.registrar.EXPECT().RegisterDependency(gomock.Any(), gomock.Any()).Return(nil, srcErr)

	_, err := m.run()
	assert.ErrorIs(t, err, srcErr)
}
