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
	manifests  *MockManifestReader
	forms      *MockFormReader
	resolver   *MockServiceResolver
	snapshots  *MockCandidateReader
	components *MockComponentReader
	publisher  *MockBuildPublisher
	uc         *PublishBuildUseCase
}

func newBuildFixture(t *testing.T) *buildFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	f := &buildFixture{
		manifests: NewMockManifestReader(ctrl),
		forms:     NewMockFormReader(ctrl),
		resolver:  NewMockServiceResolver(ctrl),
		snapshots: NewMockCandidateReader(ctrl),
		publisher: NewMockBuildPublisher(ctrl),
	}
	f.uc = NewPublishBuild(f.manifests, f.forms, f.resolver, f.snapshots, noComponents(ctrl), f.publisher)
	return f
}

// noComponents — сервис без распознаваемого состава: прочим тестам публикации
// состав безразличен.
func noComponents(ctrl *gomock.Controller) *MockComponentReader {
	reader := NewMockComponentReader(ctrl)
	reader.EXPECT().Read(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	return reader
}

func newBuildFixtureWithComponents(t *testing.T) *buildFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	f := &buildFixture{
		manifests:  NewMockManifestReader(ctrl),
		forms:      NewMockFormReader(ctrl),
		resolver:   NewMockServiceResolver(ctrl),
		snapshots:  NewMockCandidateReader(ctrl),
		components: NewMockComponentReader(ctrl),
		publisher:  NewMockBuildPublisher(ctrl),
	}
	f.uc = NewPublishBuild(f.manifests, f.forms, f.resolver, f.snapshots, f.components, f.publisher)
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

// Зависимости манифеста едут со сборкой вместе со снимками из раскладки —
// регистрирует их выкатка, уже зная версию (DEP-25).
func TestPublishBuildCarriesDependencies(t *testing.T) {
	f := newBuildFixture(t)
	manifest := &entities.Manifest{
		Service: &entities.ManifestService{Name: "paas-frontend"},
		Dependencies: []entities.ManifestDependency{{
			Name: "paas-backend", Methods: []string{"GET /services"}, Waived: []string{"Service.owner"},
		}},
	}
	f.manifests.EXPECT().Read(gomock.Any(), gomock.Any()).Return(manifest, nil)
	f.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-frontend"}).Return(map[string]string{"paas-frontend": "svc"}, nil)
	f.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "producer"}, nil)
	f.forms.EXPECT().Read(gomock.Any(), gomock.Any()).Return(nil, nil)
	dest := manifest.EffectiveDestination()
	f.snapshots.EXPECT().Read(gomock.Any(), entities.ContractSnapshotPath(dest, "paas-backend", entities.ProtocolFormatOpenAPI)).
		Return([]byte(validContract), nil)
	f.snapshots.EXPECT().Read(gomock.Any(), entities.ContractSnapshotPath(dest, "paas-backend", entities.ProtocolFormatGRPC)).
		Return(nil, os.ErrNotExist)

	var got entities.BuildRequest
	f.publisher.EXPECT().PublishBuild(gomock.Any(), "svc", gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, request entities.BuildRequest) (*entities.Build, error) {
			got = request
			return &entities.Build{ID: "build-1"}, nil
		})

	_, err := f.uc.Execute(context.Background(), PublishBuildInput{CommitRevision: "abc123"})

	require.NoError(t, err)
	assert.Equal(t, []entities.BuildDependency{{
		ProducerServiceID: "producer",
		Format:            entities.ProtocolFormatOpenAPI,
		Document:          []byte(validContract),
		Methods:           []string{"GET /services"},
		Waived:            []string{"Service.owner"},
	}}, got.Dependencies)
}

// Нет снимка в раскладке — публикация падает целиком, а не уезжает без
// зависимости: иначе выкатка молча сняла бы её из реестра.
func TestPublishBuildFailsWithoutSnapshot(t *testing.T) {
	f := newBuildFixture(t)
	manifest := &entities.Manifest{
		Service:      &entities.ManifestService{Name: "paas-frontend"},
		Dependencies: []entities.ManifestDependency{{Name: "paas-backend"}},
	}
	f.manifests.EXPECT().Read(gomock.Any(), gomock.Any()).Return(manifest, nil)
	f.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-frontend"}).Return(map[string]string{"paas-frontend": "svc"}, nil)
	f.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"paas-backend"}).Return(map[string]string{"paas-backend": "producer"}, nil)
	f.forms.EXPECT().Read(gomock.Any(), gomock.Any()).Return(nil, nil)
	f.snapshots.EXPECT().Read(gomock.Any(), gomock.Any()).Return(nil, os.ErrNotExist).Times(2)

	_, err := f.uc.Execute(context.Background(), PublishBuildInput{CommitRevision: "abc123"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "paas-backend")
}

// Состав распознаётся рядом с формой сервиса и едет со сборкой (TRA-539/AC-1).
func TestPublishBuildCarriesComponents(t *testing.T) {
	f := newBuildFixtureWithComponents(t)
	f.expectResolved(nameManifest())
	f.forms.EXPECT().Read(gomock.Any(), "svc/paas.toml").Return(nil, nil)
	components := []entities.Component{
		{Kind: entities.ComponentKindGoModule, Name: "golang.org/x/net", Version: "v0.60.0", Indirect: true},
	}
	f.components.EXPECT().Read(gomock.Any(), "svc").Return(components, nil)

	var got entities.BuildRequest
	f.publisher.EXPECT().PublishBuild(gomock.Any(), "svc", gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, request entities.BuildRequest) (*entities.Build, error) {
			got = request
			return &entities.Build{ID: "b1"}, nil
		})

	_, err := f.uc.Execute(context.Background(), PublishBuildInput{CommitRevision: "abc123", FormPath: "svc/paas.toml"})

	require.NoError(t, err)
	require.Equal(t, components, got.Components)
}

// Неразбираемый go.mod отклоняет публикацию до сети: сборка не заводится (TRA-539/AC-4).
func TestPublishBuildRejectsUnreadableComponents(t *testing.T) {
	f := newBuildFixtureWithComponents(t)
	f.expectResolved(nameManifest())
	f.forms.EXPECT().Read(gomock.Any(), gomock.Any()).Return(nil, nil)
	f.components.EXPECT().Read(gomock.Any(), ".").Return(nil, entities.ErrInvalidGoMod)

	_, err := f.uc.Execute(context.Background(), PublishBuildInput{CommitRevision: "abc123", FormPath: "paas.toml"})

	require.ErrorIs(t, err, entities.ErrInvalidGoMod)
}
