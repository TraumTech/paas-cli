package formreaderfile

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/entities"
)

func TestReadForm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paas.toml")
	require.NoError(t, os.WriteFile(path, []byte(`
[[processes]]
name = "server"
listen = 9090
command = ["./app", "serve"]
cpu = "100m"
memory = "128Mi"
zone = "main"
prefix = "api"

[processes.ports]
admin = 4434
metrics = 8003

[[processes]]
name = "worker"
`), 0o644))

	form, err := New().Read(context.Background(), path)

	require.NoError(t, err)
	require.NotNil(t, form)
	assert.Equal(t, []entities.ProcessForm{
		{Name: "server", Listen: 9090, Command: []string{"./app", "serve"}, CPU: "100m", Memory: "128Mi", Zone: "main", Prefix: "api", Ports: map[string]int{"admin": 4434, "metrics": 8003}},
		{Name: "worker"},
	}, form.Processes)
}

// Отсутствие paas.toml — штатная ветка «формы нет», не ошибка.
func TestReadFormMissingFile(t *testing.T) {
	form, err := New().Read(context.Background(), filepath.Join(t.TempDir(), "paas.toml"))

	require.NoError(t, err)
	assert.Nil(t, form)
}

func TestReadFormBadTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paas.toml")
	require.NoError(t, os.WriteFile(path, []byte("[[processes"), 0o644))

	_, err := New().Read(context.Background(), path)

	assert.Error(t, err)
}

// Единый манифест: paas.toml без [[processes]] (только [service]/deps) — это
// «формы нет», а не пустая форма, требующая образ.
func TestReadFormManifestWithoutProcesses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paas.toml")
	require.NoError(t, os.WriteFile(path, []byte("[service]\nname = \"svc\"\n"), 0o644))

	form, err := New().Read(context.Background(), path)

	require.NoError(t, err)
	assert.Nil(t, form)
}

// Объявление баз (DB-03) читается как есть: список [[databases]] и
// переопределение СУБД в секции окружения; правила — у платформы.
func TestReadFormDatabases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paas.toml")
	require.NoError(t, os.WriteFile(path, []byte(`
[[processes]]
name = "server"
listen = 9090

[[databases]]
name = "main"
engine = "postgres"
server = "paas-postgres"

[[databases]]
name = "reports"
engine = "postgres"
server = "paas-postgres"
variable = "REPORTS_DSN"

[env.dev.databases.main]
server = "dev-pg"
`), 0o600))

	declaration, err := New().Read(context.Background(), path)

	require.NoError(t, err)
	require.NotNil(t, declaration)
	assert.Equal(t, []entities.DatabaseForm{
		{Name: "main", Engine: "postgres", Server: "paas-postgres"},
		{Name: "reports", Engine: "postgres", Server: "paas-postgres", Variable: "REPORTS_DSN"},
	}, declaration.Databases)
	assert.Equal(t, map[string]entities.DatabaseOverride{"main": {Server: "dev-pg"}}, declaration.Environments["dev"].Databases)
}

// Объявление бакетов (OBJ-06) читается как есть: безымянный бакет остаётся
// с пустым именем, переопределение может менять только размер.
func TestReadFormBuckets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paas.toml")
	require.NoError(t, os.WriteFile(path, []byte(`
[[processes]]
name = "server"
listen = 9090

[[buckets]]
server = "prod-s3"
size = "20Gi"

[[buckets]]
name = "media"
server = "prod-s3"
size = "5Gi"

[env.dev.buckets.default]
server = "dev-s3"
size = "1Gi"

[env.dev.buckets.media]
size = "1Gi"
`), 0o600))

	declaration, err := New().Read(context.Background(), path)

	require.NoError(t, err)
	require.NotNil(t, declaration)
	assert.Equal(t, []entities.BucketForm{
		{Server: "prod-s3", Size: "20Gi"},
		{Name: "media", Server: "prod-s3", Size: "5Gi"},
	}, declaration.Buckets)
	assert.Equal(t, map[string]entities.BucketOverride{
		"default": {Server: "dev-s3", Size: "1Gi"},
		"media":   {Size: "1Gi"},
	}, declaration.Environments["dev"].Buckets)
}

// Секции [env.default] и [env.<окружение>] читаются как объявлены: разрешает
// их use case, зная окружение публикуемой версии (DEP-14/15).
func TestReadFormEnvironmentSections(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "paas.toml")
	require.NoError(t, os.WriteFile(path, []byte(`
[[processes]]
name = "server"
listen = 9090

[env.default]
replicas = 1

[env.default.variables]
LOG_LEVEL = "info"

[env.prod]
replicas = 2

[env.prod.variables]
LOG_LEVEL = "warn"
`), 0o600))

	declaration, err := New().Read(context.Background(), path)

	require.NoError(t, err)
	require.NotNil(t, declaration)
	assert.Equal(t, entities.EnvironmentValues{
		Variables: map[string]string{"LOG_LEVEL": "info"},
		Replicas:  1,
	}, declaration.Environments[entities.DefaultEnvironmentKey])

	form := declaration.Resolve("prod")
	assert.Equal(t, []entities.FormVariable{{Name: "LOG_LEVEL", Value: "warn"}}, form.Variables)
	assert.Equal(t, 2, form.Replicas)

	// Окружение без своей секции получает только общие значения.
	dev := declaration.Resolve("dev")
	assert.Equal(t, []entities.FormVariable{{Name: "LOG_LEVEL", Value: "info"}}, dev.Variables)
	assert.Equal(t, 1, dev.Replicas)
}
