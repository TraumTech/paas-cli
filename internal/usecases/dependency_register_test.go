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

func registerInput() RegisterDependencyInput {
	return RegisterDependencyInput{VersionID: "ver-1", ProducerServiceID: "prod", ContractPath: "contract.json", ManifestPath: "protocols.toml"}
}

func TestRegisterDependencyExecute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	reader := NewMockCandidateReader(ctrl)
	registrar := NewMockDependencyRegistrar(ctrl)

	manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(nameManifest(), nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
	reader.EXPECT().Read(gomock.Any(), "contract.json").Return([]byte(validContract), nil)
	dependency := &entities.Dependency{ConsumerVersionID: "ver-1", ProducerServiceID: "prod"}
	registrar.EXPECT().
		RegisterDependency(gomock.Any(), "svc", "ver-1", "prod", []byte(validContract)).
		Return(dependency, nil)

	got, err := NewRegisterDependency(manifests, resolver, reader, registrar).Execute(context.Background(), registerInput())

	require.NoError(t, err)
	assert.Same(t, dependency, got)
}

func TestRegisterDependencyExecute_NoServiceDeclared_NoRegister(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	reader := NewMockCandidateReader(ctrl)
	registrar := NewMockDependencyRegistrar(ctrl)

	manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(&entities.Manifest{}, nil)
	// резолвер/чтение/регистрация не вызываются — манифест не объявляет сервис.

	_, err := NewRegisterDependency(manifests, resolver, reader, registrar).Execute(context.Background(), registerInput())

	assert.ErrorIs(t, err, entities.ErrManifestNoService)
}

func TestRegisterDependencyExecute_ServiceNotFound_NoRegister(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	reader := NewMockCandidateReader(ctrl)
	registrar := NewMockDependencyRegistrar(ctrl)

	manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(nameManifest(), nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{}, nil)
	// снимок не читаем и зависимость не регистрируем — потребитель не найден.

	_, err := NewRegisterDependency(manifests, resolver, reader, registrar).Execute(context.Background(), registerInput())

	assert.ErrorIs(t, err, entities.ErrServiceNotFound)
}

func TestRegisterDependencyExecute_ReadError_NoRegister(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	reader := NewMockCandidateReader(ctrl)
	registrar := NewMockDependencyRegistrar(ctrl)

	manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(nameManifest(), nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
	readErr := errors.New("no such file")
	reader.EXPECT().Read(gomock.Any(), "contract.json").Return(nil, readErr)
	// RegisterDependency не вызывается — файл не прочитан.

	_, err := NewRegisterDependency(manifests, resolver, reader, registrar).Execute(context.Background(), registerInput())

	assert.ErrorIs(t, err, readErr)
}

func TestRegisterDependencyExecute_InvalidSnapshot_NoRegister(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	reader := NewMockCandidateReader(ctrl)
	registrar := NewMockDependencyRegistrar(ctrl)

	manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(nameManifest(), nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
	reader.EXPECT().Read(gomock.Any(), "contract.json").Return([]byte(`{"not":"openapi"}`), nil)

	_, err := NewRegisterDependency(manifests, resolver, reader, registrar).Execute(context.Background(), registerInput())

	assert.ErrorIs(t, err, entities.ErrInvalidProtocol)
}

func TestRegisterDependencyExecute_EmptySnapshot_NoRegister(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	reader := NewMockCandidateReader(ctrl)
	registrar := NewMockDependencyRegistrar(ctrl)

	manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(nameManifest(), nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
	reader.EXPECT().Read(gomock.Any(), "contract.json").Return([]byte("   "), nil)

	_, err := NewRegisterDependency(manifests, resolver, reader, registrar).Execute(context.Background(), registerInput())

	assert.ErrorIs(t, err, entities.ErrEmptyProtocol)
}

func TestRegisterDependencyExecute_SourceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	manifests := NewMockManifestReader(ctrl)
	resolver := NewMockServiceResolver(ctrl)
	reader := NewMockCandidateReader(ctrl)
	registrar := NewMockDependencyRegistrar(ctrl)

	manifests.EXPECT().Read(gomock.Any(), "protocols.toml").Return(nameManifest(), nil)
	resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
	reader.EXPECT().Read(gomock.Any(), "contract.json").Return([]byte(validContract), nil)
	srcErr := errors.New("boom")
	registrar.EXPECT().RegisterDependency(gomock.Any(), "svc", "ver-1", "prod", gomock.Any()).Return(nil, srcErr)

	_, err := NewRegisterDependency(manifests, resolver, reader, registrar).Execute(context.Background(), registerInput())

	assert.ErrorIs(t, err, srcErr)
}
