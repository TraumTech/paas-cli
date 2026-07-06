package sessionstorefile

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// withTempConfigDir уводит конфиг-директорию пользователя во временную, чтобы
// тесты не трогали настоящий сохранённый вход.
func withTempConfigDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	dir, err := os.UserConfigDir()
	require.NoError(t, err)
	return dir
}

func TestStore_SaveLoadDelete_Roundtrip(t *testing.T) {
	withTempConfigDir(t)
	store := New()
	ctx := context.Background()

	require.NoError(t, store.Save(ctx, "tok-1"))

	got, err := store.Load(ctx)
	require.NoError(t, err)
	assert.Equal(t, "tok-1", got)

	require.NoError(t, store.Delete(ctx))
	_, err = store.Load(ctx)
	assert.ErrorIs(t, err, entities.ErrNoSession)
}

func TestStore_Save_OwnerOnlyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("права POSIX не проверяются на windows")
	}
	dir := withTempConfigDir(t)
	store := New()

	require.NoError(t, store.Save(context.Background(), "tok-1"))

	info, err := os.Stat(filepath.Join(dir, "paas-cli", "session"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	dirInfo, err := os.Stat(filepath.Join(dir, "paas-cli"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())
}

func TestStore_Save_OverwritesPreviousSession(t *testing.T) {
	withTempConfigDir(t)
	store := New()
	ctx := context.Background()

	require.NoError(t, store.Save(ctx, "tok-old"))
	require.NoError(t, store.Save(ctx, "tok-new"))

	got, err := store.Load(ctx)
	require.NoError(t, err)
	assert.Equal(t, "tok-new", got)
}

func TestStore_Load_NoSession(t *testing.T) {
	withTempConfigDir(t)

	_, err := New().Load(context.Background())

	assert.ErrorIs(t, err, entities.ErrNoSession)
}

func TestStore_Delete_WithoutSession_NotAnError(t *testing.T) {
	withTempConfigDir(t)

	assert.NoError(t, New().Delete(context.Background()))
}
