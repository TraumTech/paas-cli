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

// gRPC-контракт кладётся в родном виде: contract.proto, текст как есть.
func TestStoreSave_GRPCWritesProtoAsIs(t *testing.T) {
	destDir := t.TempDir()

	proto := "syntax = \"proto3\";\npackage traumtech.paas_protocols.v1;"
	protocol := &entities.Protocol{
		ServiceName: "paas-protocols",
		Format:      entities.ProtocolFormatGRPC,
		Document:    []byte(proto),
	}
	path, err := protocolstorefile.New().Save(context.Background(), protocol, destDir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(destDir, "paas-protocols", "contract.proto"), path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, proto+"\n", string(data), ".proto пишется как есть, с переводом строки в конце")
}

// Повторная запись того же .proto воспроизводима байт в байт.
func TestStoreSave_GRPCIdempotent(t *testing.T) {
	destDir := t.TempDir()
	protocol := &entities.Protocol{
		ServiceName: "paas-protocols",
		Format:      entities.ProtocolFormatGRPC,
		Document:    []byte("syntax = \"proto3\";\n"),
	}
	store := protocolstorefile.New()
	path1, err := store.Save(context.Background(), protocol, destDir)
	require.NoError(t, err)
	first, err := os.ReadFile(path1)
	require.NoError(t, err)

	path2, err := store.Save(context.Background(), protocol, destDir)
	require.NoError(t, err)
	second, err := os.ReadFile(path2)
	require.NoError(t, err)

	assert.Equal(t, path1, path2)
	assert.Equal(t, first, second)
}
