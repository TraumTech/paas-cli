package protocolstorefile_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/adapters/protocol_store_file"
	"github.com/TraumTech/paas-cli/internal/entities"
)

func TestStoreSave_WritesToServiceDir(t *testing.T) {
	destDir := t.TempDir()

	protocol := &entities.Protocol{ServiceName: "payments", Document: []byte(`{"openapi":"3.1.0","paths":{}}`)}
	path, err := protocolstorefile.New().Save(context.Background(), protocol, destDir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(destDir, "payments", "openapi.json"), path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.JSONEq(t, `{"openapi":"3.1.0","paths":{}}`, string(data))
	assert.Contains(t, string(data), "\n  ", "документ должен быть с отступами")
}

func TestStoreSave_DoesNotClobberOnInvalidJSON(t *testing.T) {
	destDir := t.TempDir()
	svcDir := filepath.Join(destDir, "payments")
	require.NoError(t, os.MkdirAll(svcDir, 0o755))
	dest := filepath.Join(svcDir, "openapi.json")
	previous := []byte("PREVIOUS GOOD CONTRACT")
	require.NoError(t, os.WriteFile(dest, previous, 0o644))

	// json.Indent падает на битом документе — целевой файл не должен пострадать.
	_, err := protocolstorefile.New().Save(context.Background(),
		&entities.Protocol{ServiceName: "payments", Document: []byte("not json")}, destDir)
	require.Error(t, err)

	data, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, previous, data, "рабочий контракт не затёрт")

	entries, err := os.ReadDir(svcDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "временный файл подчищён")
}
