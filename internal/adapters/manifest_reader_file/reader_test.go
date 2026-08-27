package manifestreaderfile_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/adapters/manifest_reader_file"
	"github.com/TraumTech/paas-cli/internal/entities"
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
methods = ["GET /a", "POST /b"]
`)

	got, err := manifestreaderfile.New().Read(context.Background(), path)
	require.NoError(t, err)
	assert.Equal(t, "vendor/api", got.Destination)
	require.Len(t, got.Dependencies, 2)
	assert.Equal(t, "paas-backend", got.Dependencies[0].Name)
	assert.Empty(t, got.Dependencies[0].Methods)
	assert.Equal(t, "billing", got.Dependencies[1].Name)
	assert.Equal(t, []string{"GET /a", "POST /b"}, got.Dependencies[1].Methods)
}

// PRT-26: отказы от атрибутов читаются рядом с методами; их отсутствие — пусто.
func TestRead_Attributes(t *testing.T) {
	path := writeManifest(t, `
[[dependencies]]
name = "paas-backend"
methods = ["GET /services"]
attributes = ["GET /services#response.200.name"]
`)

	got, err := manifestreaderfile.New().Read(context.Background(), path)
	require.NoError(t, err)
	require.Len(t, got.Dependencies, 1)
	assert.Equal(t, []string{"GET /services#response.200.name"}, got.Dependencies[0].Attributes)
}

func TestRead_WaivedAttributes(t *testing.T) {
	path := writeManifest(t, `
[[dependencies]]
name = "paas-backend"

[[dependencies]]
name = "paas-services"
waived = ["traumtech.paas_services.v1.Service.owner_id"]
`)

	got, err := manifestreaderfile.New().Read(context.Background(), path)
	require.NoError(t, err)
	require.Len(t, got.Dependencies, 2)
	assert.Empty(t, got.Dependencies[0].Waived)
	assert.Equal(t, []string{"traumtech.paas_services.v1.Service.owner_id"}, got.Dependencies[1].Waived)
}

func TestRead_Service(t *testing.T) {
	path := writeManifest(t, `
[service]
name = "paas-backend"
contract = "api/openapi.json"

[[dependencies]]
name = "billing"
`)

	got, err := manifestreaderfile.New().Read(context.Background(), path)
	require.NoError(t, err)
	require.NotNil(t, got.Service)
	assert.Equal(t, "paas-backend", got.Service.Name)
	assert.Equal(t, "api/openapi.json", got.Service.Contract)
	assert.Empty(t, got.Service.Format)
}

// Перечень [[protocols]] (CLI-23) читается записями с именем, контрактом и
// форматом рядом с секцией [service].
func TestRead_Protocols(t *testing.T) {
	path := writeManifest(t, `
[service]
name = "paas-backend"

[[protocols]]
name = "http"
contract = "openapi.json"

[[protocols]]
name = "internal-grpc"
contract = "api/edge.proto"
format = "grpc"
`)

	got, err := manifestreaderfile.New().Read(context.Background(), path)
	require.NoError(t, err)
	require.Len(t, got.Protocols, 2)
	assert.Equal(t, entities.ManifestProtocol{Name: "http", Contract: "openapi.json"}, got.Protocols[0])
	assert.Equal(t, entities.ManifestProtocol{Name: "internal-grpc", Contract: "api/edge.proto", Format: "grpc"}, got.Protocols[1])
}

func TestRead_ServiceFormat(t *testing.T) {
	path := writeManifest(t, `
[service]
name = "paas-protocols"
contract = "internal/infrastructure/grpc/registry.proto"
format = "grpc"

[[dependencies]]
name = "paas-backend"
`)

	got, err := manifestreaderfile.New().Read(context.Background(), path)
	require.NoError(t, err)
	require.NotNil(t, got.Service)
	assert.Equal(t, "grpc", got.Service.Format)
}

// Reader только разбирает TOML и не валидирует: файл без [service] читается с
// Service == nil, а обязательность секции проверяет Manifest.Validate.
func TestRead_NoServiceParsesNil(t *testing.T) {
	path := writeManifest(t, `
[[dependencies]]
name = "paas-backend"
`)

	got, err := manifestreaderfile.New().Read(context.Background(), path)
	require.NoError(t, err)
	assert.Nil(t, got.Service)
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

// Резолюция манифеста без явного пути (CLI-22): paas.toml предпочитается,
// protocols.toml остаётся переходным, полупереезд — явная ошибка.
func TestRead_ResolvesUnifiedManifest(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.WriteFile("paas.toml", []byte("[service]\nname = \"svc-new\"\n"), 0o644))
	require.NoError(t, os.WriteFile("protocols.toml", []byte("[service]\nname = \"svc-old\"\n"), 0o644))

	got, err := manifestreaderfile.New().Read(context.Background(), "")
	require.NoError(t, err)
	name, err := got.ServiceName()
	require.NoError(t, err)
	assert.Equal(t, "svc-new", name)
}

func TestRead_FallsBackToLegacyManifest(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.WriteFile("protocols.toml", []byte("[service]\nname = \"svc-old\"\n"), 0o644))

	got, err := manifestreaderfile.New().Read(context.Background(), "")
	require.NoError(t, err)
	name, err := got.ServiceName()
	require.NoError(t, err)
	assert.Equal(t, "svc-old", name)
}

// paas.toml есть (форма), но секции манифеста остались в protocols.toml —
// молча читать старый файл нельзя: правки нового молча не действовали бы.
func TestRead_HalfMigratedIsExplicitError(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.WriteFile("paas.toml", []byte("[[processes]]\nname = \"server\"\n"), 0o644))
	require.NoError(t, os.WriteFile("protocols.toml", []byte("[service]\nname = \"svc-old\"\n"), 0o644))

	_, err := manifestreaderfile.New().Read(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "перенесите")
}
