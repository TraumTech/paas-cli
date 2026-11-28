package servicefile

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var out bytes.Buffer
	previous := Warnings
	Warnings, warnOnce = &out, sync.Once{}
	t.Cleanup(func() { Warnings = previous })
	return &out
}

func touch(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, nil, 0o644))
	return path
}

// TRA-555/AC-1
func TestResolve_Current(t *testing.T) {
	out, dir := captureWarnings(t), t.TempDir()
	want := touch(t, dir, Name)

	path, found := Resolve(dir)
	assert.True(t, found)
	assert.Equal(t, want, path)
	assert.Empty(t, out.String())
}

// TRA-555/AC-2
func TestResolve_FormerNameStillRead(t *testing.T) {
	out, dir := captureWarnings(t), t.TempDir()
	want := touch(t, dir, FormerName)

	path, found := Resolve(dir)
	assert.True(t, found)
	assert.Equal(t, want, path)
	assert.Empty(t, out.String())
}

// TRA-555/AC-3
func TestResolve_BothPrefersCurrentAndWarnsOnce(t *testing.T) {
	out, dir := captureWarnings(t), t.TempDir()
	want := touch(t, dir, Name)
	touch(t, dir, FormerName)

	path, found := Resolve(dir)
	Resolve(dir)
	assert.True(t, found)
	assert.Equal(t, want, path)
	assert.Equal(t, 1, bytes.Count(out.Bytes(), []byte("\n")))
	assert.Contains(t, out.String(), "paas.toml не используется")
}

func TestResolve_None(t *testing.T) {
	captureWarnings(t)
	dir := t.TempDir()
	path, found := Resolve(dir)
	assert.False(t, found)
	assert.Equal(t, filepath.Join(dir, Name), path)
}
