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
	m.registrar.EXPECT().
		RegisterDependency(gomock.Any(), "consumer", "ver-1", "prod", []byte(validContract), false).
		Return(&entities.Dependency{ConsumerVersionID: "ver-1", ProducerServiceID: "prod"}, nil)

	got, err := m.run()

	require.NoError(t, err)
	require.Len(t, got.Registered, 1)
	assert.Equal(t, "paas-backend", got.Registered[0].ProducerName)
	assert.Equal(t, "prod", got.Registered[0].ProducerServiceID)
}

// С SupersedePrevious флаг замещения уходит в каждый вызов регистрации.
func TestRegisterDependencyExecute_SupersedePrevious(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newRegisterMocks(ctrl)

	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(consumerManifest("paas-backend"), nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-frontend", "paas-backend"}).
		Return(map[string]string{"paas-frontend": "consumer", "paas-backend": "prod"}, nil)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-backend/openapi.json").Return([]byte(validContract), nil)
	m.registrar.EXPECT().
		RegisterDependency(gomock.Any(), "consumer", "ver-1", "prod", []byte(validContract), true).
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
	m.reader.EXPECT().Read(gomock.Any(), "protocols/billing/openapi.json").Return([]byte(validContract), nil)
	m.registrar.EXPECT().RegisterDependency(gomock.Any(), "consumer", "ver-1", "prod-a", gomock.Any(), false).Return(&entities.Dependency{}, nil)
	m.registrar.EXPECT().RegisterDependency(gomock.Any(), "consumer", "ver-1", "prod-b", gomock.Any(), false).Return(&entities.Dependency{}, nil)

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
	m.registrar.EXPECT().RegisterDependency(gomock.Any(), "consumer", "ver-1", "prod", gomock.Any(), false).Return(&entities.Dependency{}, nil)

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

	_, err := m.run()
	assert.ErrorIs(t, err, entities.ErrInvalidProtocol)
}

func TestRegisterDependencyExecute_RegistrarError_Aborts(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newRegisterMocks(ctrl)

	m.manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(consumerManifest("paas-backend"), nil)
	m.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-frontend", "paas-backend"}).
		Return(map[string]string{"paas-frontend": "consumer", "paas-backend": "prod"}, nil)
	m.reader.EXPECT().Read(gomock.Any(), "protocols/paas-backend/openapi.json").Return([]byte(validContract), nil)
	srcErr := errors.New("boom")
	m.registrar.EXPECT().RegisterDependency(gomock.Any(), "consumer", "ver-1", "prod", gomock.Any(), false).Return(nil, srcErr)

	_, err := m.run()
	assert.ErrorIs(t, err, srcErr)
}
