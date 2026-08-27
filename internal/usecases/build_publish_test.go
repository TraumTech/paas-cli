package usecases

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/TraumTech/paas-cli/internal/entities"
)

type buildFixture struct {
	manifests *MockManifestReader
	forms     *MockFormReader
	resolver  *MockServiceResolver
	publisher *MockBuildPublisher
	uc        *PublishBuildUseCase
}

func newBuildFixture(t *testing.T) *buildFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	f := &buildFixture{
		manifests: NewMockManifestReader(ctrl),
		forms:     NewMockFormReader(ctrl),
		resolver:  NewMockServiceResolver(ctrl),
		publisher: NewMockBuildPublisher(ctrl),
	}
	f.uc = NewPublishBuild(f.manifests, f.forms, f.resolver, f.publisher)
	return f
}

func (f *buildFixture) expectResolved(manifest *entities.Manifest) {
	f.manifests.EXPECT().Read(gomock.Any(), gomock.Any()).Return(manifest, nil)
	f.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "svc"}, nil)
}

// Секции окружений едут неразрешёнными: сливает их выкатка, когда окружение
// выбрано (в отличие от публикации версии, где сливал публикующий).
func TestPublishBuildKeepsEnvironmentSections(t *testing.T) {
	f := newBuildFixture(t)
	f.expectResolved(nameManifest())
	declaration := &entities.FormDeclaration{
		Processes: []entities.ProcessForm{{Name: "server", Listen: 8080}},
		Environments: map[string]entities.EnvironmentValues{
			entities.DefaultEnvironmentKey: {Variables: map[string]string{"LOG_LEVEL": "info"}},
			"prod":                         {Replicas: 2},
		},
	}
	f.forms.EXPECT().Read(gomock.Any(), "paas.toml").Return(declaration, nil)

	var got entities.BuildRequest
	f.publisher.EXPECT().PublishBuild(gomock.Any(), "svc", gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, request entities.BuildRequest) (*entities.Build, error) {
			got = request
			return &entities.Build{ID: "build-1", CommitRevision: "abc123", Branch: "feature/x"}, nil
		})

	build, err := f.uc.Execute(context.Background(), PublishBuildInput{
		CommitRevision: "abc123",
		Branch:         "feature/x",
		FormPath:       "paas.toml",
		Image:          "img",
	})

	require.NoError(t, err)
	assert.Equal(t, "build-1", build.ID)
	assert.Equal(t, declaration, got.Form)
	assert.Equal(t, "feature/x", got.Branch)
}

// Контракт едет с артефактом: в реестр его публикует выкатка (DEP-19).
func TestPublishBuildCarriesContract(t *testing.T) {
	f := newBuildFixture(t)
	dir := t.TempDir()
	contract := filepath.Join(dir, "contract.proto")
	require.NoError(t, os.WriteFile(contract, []byte("syntax = \"proto3\";"), 0o600))

	manifest := &entities.Manifest{Service: &entities.ManifestService{
		Name: "paas-backend", Contract: contract, Format: "grpc",
	}}
	f.expectResolved(manifest)
	f.forms.EXPECT().Read(gomock.Any(), gomock.Any()).Return(nil, nil)

	var got entities.BuildRequest
	f.publisher.EXPECT().PublishBuild(gomock.Any(), "svc", gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, request entities.BuildRequest) (*entities.Build, error) {
			got = request
			return &entities.Build{ID: "build-1"}, nil
		})

	_, err := f.uc.Execute(context.Background(), PublishBuildInput{CommitRevision: "abc123"})

	require.NoError(t, err)
	assert.Equal(t, "syntax = \"proto3\";", got.Contract)
	assert.Equal(t, "grpc", got.ContractFormat)
}

// Форма без образа не имеет смысла — отказываем до сети, как при публикации версии.
func TestPublishBuildRejectsFormWithoutImage(t *testing.T) {
	f := newBuildFixture(t)
	f.expectResolved(nameManifest())
	f.forms.EXPECT().Read(gomock.Any(), gomock.Any()).Return(&entities.FormDeclaration{
		Processes: []entities.ProcessForm{{Name: "server", Listen: 8080}},
	}, nil)

	_, err := f.uc.Execute(context.Background(), PublishBuildInput{CommitRevision: "abc123"})

	assert.ErrorIs(t, err, entities.ErrFormRequiresImage)
}

// Сборка несёт один контракт без имени: перечень [[protocols]] из нескольких
// или именованных записей — понятный отказ до сети (CLI-23), а единственная
// запись с именем по умолчанию работает как прежняя форма.
func TestPublishBuildRejectsMultipleContracts(t *testing.T) {
	f := newBuildFixture(t)
	manifest := &entities.Manifest{
		Service: &entities.ManifestService{Name: "paas-backend"},
		Protocols: []entities.ManifestProtocol{
			{Name: "http", Contract: "a.json"},
			{Name: "admin", Contract: "b.json"},
		},
	}
	f.expectResolved(manifest)
	f.forms.EXPECT().Read(gomock.Any(), gomock.Any()).Return(nil, nil)

	_, err := f.uc.Execute(context.Background(), PublishBuildInput{CommitRevision: "abc123"})

	assert.ErrorIs(t, err, entities.ErrBuildMultipleContracts)
}

func TestPublishBuildRejectsNamedContract(t *testing.T) {
	f := newBuildFixture(t)
	manifest := &entities.Manifest{
		Service:   &entities.ManifestService{Name: "paas-backend"},
		Protocols: []entities.ManifestProtocol{{Name: "http", Contract: "a.json"}},
	}
	f.expectResolved(manifest)
	f.forms.EXPECT().Read(gomock.Any(), gomock.Any()).Return(nil, nil)

	_, err := f.uc.Execute(context.Background(), PublishBuildInput{CommitRevision: "abc123"})

	assert.ErrorIs(t, err, entities.ErrBuildNamedContract)
}

func TestPublishBuildCarriesDefaultNamedContract(t *testing.T) {
	f := newBuildFixture(t)
	dir := t.TempDir()
	contract := filepath.Join(dir, "openapi.json")
	require.NoError(t, os.WriteFile(contract, []byte(`{"openapi":"3.1.0"}`), 0o600))

	manifest := &entities.Manifest{
		Service:   &entities.ManifestService{Name: "paas-backend"},
		Protocols: []entities.ManifestProtocol{{Name: entities.DefaultProtocolName, Contract: contract}},
	}
	f.expectResolved(manifest)
	f.forms.EXPECT().Read(gomock.Any(), gomock.Any()).Return(nil, nil)

	var got entities.BuildRequest
	f.publisher.EXPECT().PublishBuild(gomock.Any(), "svc", gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, request entities.BuildRequest) (*entities.Build, error) {
			got = request
			return &entities.Build{ID: "build-1"}, nil
		})

	_, err := f.uc.Execute(context.Background(), PublishBuildInput{CommitRevision: "abc123"})

	require.NoError(t, err)
	assert.Equal(t, `{"openapi":"3.1.0"}`, got.Contract)
	assert.Equal(t, "openapi", got.ContractFormat)
}
