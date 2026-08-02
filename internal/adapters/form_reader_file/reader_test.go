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

[[processes]]
name = "worker"
`), 0o644))

	form, err := New().Read(context.Background(), path)

	require.NoError(t, err)
	require.NotNil(t, form)
	assert.Equal(t, []entities.ProcessForm{
		{Name: "server", Listen: 9090, Command: []string{"./app", "serve"}, CPU: "100m", Memory: "128Mi"},
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
