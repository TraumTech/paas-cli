package candidatereaderfile_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/adapters/candidate_reader_file"
)

func TestRead_ReturnsFileContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "openapi.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"openapi":"3.1.0"}`), 0o644))

	got, err := candidatereaderfile.New().Read(context.Background(), path)
	require.NoError(t, err)
	assert.JSONEq(t, `{"openapi":"3.1.0"}`, string(got))
}

func TestRead_MissingFile(t *testing.T) {
	_, err := candidatereaderfile.New().Read(context.Background(), filepath.Join(t.TempDir(), "absent.json"))
	require.Error(t, err)
}
