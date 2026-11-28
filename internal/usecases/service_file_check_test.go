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

type fakeInventory struct {
	inventory entities.Inventory
	calls     int
}

func (f *fakeInventory) Inventory(context.Context) (*entities.Inventory, error) {
	f.calls++
	return &f.inventory, nil
}

type checkFixture struct {
	manifests *MockManifestReader
	forms     *MockFormReader
	resolver  *MockServiceResolver
	inventory *fakeInventory
	path      string
}

func newCheckFixture(t *testing.T) *checkFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	path := filepath.Join(t.TempDir(), "stapel.toml")
	require.NoError(t, os.WriteFile(path, nil, 0o644))
	return &checkFixture{
		manifests: NewMockManifestReader(ctrl),
		forms:     NewMockFormReader(ctrl),
		resolver:  NewMockServiceResolver(ctrl),
		inventory: &fakeInventory{},
		path:      path,
	}
}

func (f *checkFixture) run(signedIn bool) (*CheckServiceFileResult, error) {
	uc := NewCheckServiceFile(f.manifests, f.forms, f.resolver, f.inventory, signedIn)
	return uc.Execute(context.Background(), CheckServiceFileInput{Path: f.path})
}

func (f *checkFixture) file(manifest *entities.Manifest, form *entities.FormDeclaration) {
	f.manifests.EXPECT().Read(gomock.Any(), f.path).Return(manifest, nil)
	f.forms.EXPECT().Read(gomock.Any(), f.path).Return(form, nil)
}

var checkedManifest = &entities.Manifest{Service: &entities.ManifestService{Name: "svc"}}

// TRA-556/AC-4: без входа файл проверяется локально, платформа не трогается.
func TestCheckServiceFile_NotSignedInSkipsPlatform(t *testing.T) {
	f := newCheckFixture(t)
	f.file(checkedManifest, &entities.FormDeclaration{
		Processes:    []entities.ProcessForm{{Name: "svc", Zone: "nowhere"}},
		Environments: map[string]entities.EnvironmentValues{"staging": {}},
	})

	result, err := f.run(false)

	require.NoError(t, err)
	assert.True(t, result.PlatformSkipped)
	assert.Empty(t, result.Missing)
	assert.Zero(t, f.inventory.calls)
}

// TRA-556/AC-1: вход выполнен, всё названное в файле заведено.
func TestCheckServiceFile_AllReferencesExist(t *testing.T) {
	f := newCheckFixture(t)
	f.file(checkedManifest, &entities.FormDeclaration{
		Processes:    []entities.ProcessForm{{Name: "svc", Zone: "main"}},
		Databases:    []entities.DatabaseForm{{Name: "main", Server: "pg"}},
		Environments: map[string]entities.EnvironmentValues{"default": {}, "prod": {}},
	})
	f.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"svc"}).Return(map[string]string{"svc": "id"}, nil)
	f.inventory.inventory = entities.Inventory{Environments: []string{"prod"}, Zones: []string{"main"}, Servers: []string{"pg"}}

	result, err := f.run(true)

	require.NoError(t, err)
	assert.False(t, result.PlatformSkipped)
	assert.Empty(t, result.Missing)
}

// TRA-556/AC-3: названо всё, чего нет, и где это завести.
func TestCheckServiceFile_NamesEverythingMissing(t *testing.T) {
	f := newCheckFixture(t)
	f.file(checkedManifest, &entities.FormDeclaration{
		Processes: []entities.ProcessForm{{
			Name: "svc", Zone: "main",
			Routes: []entities.RouteForm{{Port: "admin", Zone: "inner"}},
		}},
		Databases: []entities.DatabaseForm{{Name: "main", Server: "pg"}},
		Buckets:   []entities.BucketForm{{Name: "files", Server: "s3"}},
		Environments: map[string]entities.EnvironmentValues{
			"staging": {Databases: map[string]entities.DatabaseOverride{"main": {Server: "pg-staging"}}},
		},
	})
	f.resolver.EXPECT().ResolveIDs(gomock.Any(), []string{"svc"}).Return(map[string]string{}, nil)
	f.inventory.inventory = entities.Inventory{Environments: []string{"prod"}, Zones: []string{"main"}, Servers: []string{"pg"}}

	result, err := f.run(true)

	require.NoError(t, err)
	lines := make([]string, 0, len(result.Missing))
	for _, m := range result.Missing {
		lines = append(lines, m.String())
	}
	assert.Equal(t, []string{
		"сервис svc (name в [service]) — в организации такого нет; сервисы заводятся на https://stapel.sh",
		"окружение staging (секция [env.staging]) — в организации такого нет; окружения заводятся на https://stapel.sh/clusters",
		`доменная зона inner (zone входа на порт "admin" процесса "svc") — в организации такого нет; зоны заводятся на https://stapel.sh/domains`,
		`СУБД или хранилище s3 (server бакета "files") — в организации такого нет; СУБД и хранилища подключаются на https://stapel.sh/databases`,
		"СУБД или хранилище pg-staging (server в [env.staging.databases.main]) — в организации такого нет; СУБД и хранилища подключаются на https://stapel.sh/databases",
	}, lines)
}

// TRA-556/AC-6: файла нет — сказано, какой ожидается и где.
func TestCheckServiceFile_MissingFile(t *testing.T) {
	f := newCheckFixture(t)
	require.NoError(t, os.Remove(f.path))

	_, err := f.run(true)

	var missing *entities.ServiceFileMissingError
	require.ErrorAs(t, err, &missing)
	assert.Equal(t, "stapel.toml", missing.Expected)
	assert.Equal(t, filepath.Dir(f.path), missing.Dir)
}

// TRA-556/AC-2: локальная ошибка та же, что при публикации сборки.
func TestCheckServiceFile_LocalRulesOfBuildPublish(t *testing.T) {
	f := newCheckFixture(t)
	f.manifests.EXPECT().Read(gomock.Any(), f.path).Return(&entities.Manifest{
		Service:   &entities.ManifestService{Name: "svc"},
		Protocols: []entities.ManifestProtocol{{Name: "a", Contract: "a.json"}, {Name: "b", Contract: "b.json"}},
	}, nil)

	_, err := f.run(true)

	assert.ErrorIs(t, err, entities.ErrBuildMultipleContracts)
}

// Контракт, собираемый в CI, в рабочей копии отсутствует — замечание, не ошибка.
func TestCheckServiceFile_AbsentContractIsNote(t *testing.T) {
	f := newCheckFixture(t)
	f.file(&entities.Manifest{Service: &entities.ManifestService{Name: "svc", Contract: filepath.Join(t.TempDir(), "openapi.json")}}, nil)

	result, err := f.run(false)

	require.NoError(t, err)
	assert.Len(t, result.Notes, 1)
}
