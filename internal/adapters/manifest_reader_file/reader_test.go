package manifestreaderfile_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/adapters/manifest_reader_file"
)

func writeManifest(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "protocols.toml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}

func TestRead_FullAndPartial(t *testing.T) {
	path := writeManifest(t, `
destination = "vendor/api"

[[dependencies]]
name = "paas-backend"

[[dependencies]]
name = "billing"
methods = ["op-a", "op-b"]
`)

	got, err := manifestreaderfile.New().Read(context.Background(), path)
	require.NoError(t, err)
	assert.Equal(t, "vendor/api", got.Destination)
	require.Len(t, got.Dependencies, 2)
	assert.Equal(t, "paas-backend", got.Dependencies[0].Name)
	assert.Empty(t, got.Dependencies[0].Methods)
	assert.Equal(t, "billing", got.Dependencies[1].Name)
	assert.Equal(t, []string{"op-a", "op-b"}, got.Dependencies[1].Methods)
}

func TestRead_DestinationOptional(t *testing.T) {
	path := writeManifest(t, `
[[dependencies]]
name = "paas-backend"
`)

	got, err := manifestreaderfile.New().Read(context.Background(), path)
	require.NoError(t, err)
	assert.Empty(t, got.Destination)
	assert.Equal(t, "protocols", got.EffectiveDestination())
}

func TestRead_MissingFile(t *testing.T) {
	_, err := manifestreaderfile.New().Read(context.Background(), filepath.Join(t.TempDir(), "absent.toml"))
	require.Error(t, err)
}

func TestRead_InvalidTOML(t *testing.T) {
	path := writeManifest(t, "this is = = not toml")
	_, err := manifestreaderfile.New().Read(context.Background(), path)
	require.Error(t, err)
}
