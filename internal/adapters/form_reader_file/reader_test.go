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

// Дополнительные входы на внутренние порты и потоковость (TRA-509).
func TestReadFormRoutes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paas.toml")
	require.NoError(t, os.WriteFile(path, []byte(`
[[processes]]
name = "web"
listen = 8080
command = ["/app"]
zone = "nomadvisa"
prefix = "api"

[processes.ports]
mcp = 8081

[[processes.routes]]
port = "mcp"
zone = "nomadvisa"
prefix = "mcp"
streaming = true
`), 0o644))

	form, err := New().Read(context.Background(), path)

	require.NoError(t, err)
	require.Len(t, form.Processes, 1)
	assert.False(t, form.Processes[0].Streaming)
	assert.Equal(t, []entities.RouteForm{{Port: "mcp", Zone: "nomadvisa", Prefix: "mcp", Streaming: true}}, form.Processes[0].Routes)
}

// Дополнительные адреса основного и дополнительного входов (TRA-530).
func TestReadFormAliases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paas.toml")
	require.NoError(t, os.WriteFile(path, []byte(`
[[processes]]
name = "web"
listen = 8080
command = ["/app"]
zone = "old"
prefix = "api"

[[processes.aliases]]
zone = "new"
prefix = "api"

[[processes.aliases]]
zone = "new"

[processes.ports]
mcp = 8081

[[processes.routes]]
port = "mcp"
zone = "old"
prefix = "mcp"

[[processes.routes.aliases]]
zone = "new"
prefix = "mcp"
`), 0o644))

	form, err := New().Read(context.Background(), path)

	require.NoError(t, err)
	require.Len(t, form.Processes, 1)
	assert.Equal(t, []entities.RouteAlias{{Zone: "new", Prefix: "api"}, {Zone: "new"}}, form.Processes[0].Aliases)
	assert.Equal(t, []entities.RouteForm{{Port: "mcp", Zone: "old", Prefix: "mcp",
		Aliases: []entities.RouteAlias{{Zone: "new", Prefix: "mcp"}}}}, form.Processes[0].Routes)
}

// Правила доступа основного и дополнительного входов (TRA-518).
func TestReadFormAccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paas.toml")
	require.NoError(t, os.WriteFile(path, []byte(`
[[processes]]
name = "web"
listen = 8080
command = ["/app"]
zone = "main"
prefix = "api"

[processes.ports]
mcp = 8081

[[processes.access]]
paths = ["/docs", "/schemas/**"]
methods = ["GET"]
anonymous = true

[[processes.access]]
paths = ["/users/*/orders"]
sources = ["paas"]

[[processes.access]]
paths = ["/registry/token"]
methods = ["GET"]
passthrough = true

[[processes.routes]]
port = "mcp"
zone = "main"
prefix = "mcp"

[[processes.routes.access]]
paths = ["/**"]
sources = ["paas"]
anonymous = true
`), 0o644))

	form, err := New().Read(context.Background(), path)

	require.NoError(t, err)
	require.Len(t, form.Processes, 1)
	assert.Equal(t, []entities.RouteAccess{
		{Paths: []string{"/docs", "/schemas/**"}, Methods: []string{"GET"}, Anonymous: true},
		{Paths: []string{"/users/*/orders"}, Sources: []string{"paas"}},
		{Paths: []string{"/registry/token"}, Methods: []string{"GET"}, Passthrough: true},
	}, form.Processes[0].Access)
	require.Len(t, form.Processes[0].Routes, 1)
	assert.Equal(t, []entities.RouteAccess{
		{Paths: []string{"/**"}, Sources: []string{"paas"}, Anonymous: true},
	}, form.Processes[0].Routes[0].Access)
}

// CORS основного и дополнительного входов (TRA-524).
func TestReadFormCORS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paas.toml")
	require.NoError(t, os.WriteFile(path, []byte(`
[[processes]]
name = "web"
listen = 8080
command = ["/app"]
zone = "main"
prefix = "api"

[processes.ports]
mcp = 8081

[processes.cors]
origins = ["https://app.example.com"]
headers = ["traceparent", "X-Organization-Id"]

[[processes.routes]]
port = "mcp"
zone = "main"
prefix = "mcp"

[processes.routes.cors]
origins = ["https://agent.example.com"]
`), 0o644))

	form, err := New().Read(context.Background(), path)

	require.NoError(t, err)
	require.Len(t, form.Processes, 1)
	assert.Equal(t, entities.RouteCORS{Origins: []string{"https://app.example.com"}, Headers: []string{"traceparent", "X-Organization-Id"}}, form.Processes[0].CORS)
	require.Len(t, form.Processes[0].Routes, 1)
	assert.Equal(t, entities.RouteCORS{Origins: []string{"https://agent.example.com"}}, form.Processes[0].Routes[0].CORS)
}

// TRA-555/AC-1…AC-3: без явного пути форма берётся из stapel.toml, при его
// отсутствии — из прежнего paas.toml.
func TestReadFormResolvesServiceFile(t *testing.T) {
	form := func(name string) []byte {
		return []byte("[[processes]]\nname = \"" + name + "\"\ncommand = [\"/app\"]\n")
	}
	cases := map[string]struct {
		files map[string]string
		want  string
	}{
		"stapel.toml":       {files: map[string]string{"stapel.toml": "current"}, want: "current"},
		"прежний paas.toml": {files: map[string]string{"paas.toml": "former"}, want: "former"},
		"оба файла":         {files: map[string]string{"stapel.toml": "current", "paas.toml": "former"}, want: "current"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			for file, process := range tc.files {
				require.NoError(t, os.WriteFile(file, form(process), 0o644))
			}
			got, err := New().Read(context.Background(), "")
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tc.want, got.Processes[0].Name)
		})
	}
}

func TestReadFormNoServiceFile(t *testing.T) {
	t.Chdir(t.TempDir())
	got, err := New().Read(context.Background(), "")
	require.NoError(t, err)
	assert.Nil(t, got)
}
