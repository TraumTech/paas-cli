package sessionstorefile

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

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

	require.NoError(t, store.Save(ctx, entities.Credential{
		Kind:  entities.CredentialSession,
		Token: "tok-1",
		Email: "user@example.com",
	}))

	got, err := store.Load(ctx)
	require.NoError(t, err)
	assert.Equal(t, entities.CredentialSession, got.Kind)
	assert.Equal(t, "tok-1", got.Token)
	assert.Equal(t, "user@example.com", got.Email)

	require.NoError(t, store.Delete(ctx))
	_, err = store.Load(ctx)
	assert.ErrorIs(t, err, entities.ErrNoSession)
}

// Личный токен (AUTH-22) помнит и то, чем его отзывать при выходе, и докуда он
// действует: восстановить это из самого секрета нельзя.
func TestStore_SaveLoad_PersonalToken(t *testing.T) {
	withTempConfigDir(t)
	store := New()
	ctx := context.Background()
	expiresAt := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

	require.NoError(t, store.Save(ctx, entities.Credential{
		Kind:      entities.CredentialPersonalToken,
		Token:     "paas_uat_secret",
		TokenID:   "01a00030-f945-774b-b5c0-e96f81a16580",
		Email:     "user@example.com",
		ExpiresAt: expiresAt,
	}))

	got, err := store.Load(ctx)
	require.NoError(t, err)
	assert.Equal(t, entities.CredentialPersonalToken, got.Kind)
	assert.Equal(t, "paas_uat_secret", got.Token)
	assert.Equal(t, "01a00030-f945-774b-b5c0-e96f81a16580", got.TokenID)
	assert.True(t, expiresAt.Equal(got.ExpiresAt))
}

// Вход, сохранённый прежними версиями CLI, — голая строка токена сессии:
// обновление CLI не должно выкидывать пользователя.
func TestStore_Load_LegacyPlainToken(t *testing.T) {
	dir := withTempConfigDir(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "paas-cli"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "paas-cli", "session"), []byte("ory_st_legacy\n"), 0o600))

	got, err := New().Load(context.Background())

	require.NoError(t, err)
	assert.Equal(t, entities.CredentialSession, got.Kind)
	assert.Equal(t, "ory_st_legacy", got.Token)
}

func TestStore_Save_OwnerOnlyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("права POSIX не проверяются на windows")
	}
	dir := withTempConfigDir(t)
	store := New()

	require.NoError(t, store.Save(context.Background(), entities.Credential{
		Kind: entities.CredentialSession, Token: "tok-1",
	}))

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

	require.NoError(t, store.Save(ctx, entities.Credential{Kind: entities.CredentialSession, Token: "tok-old"}))
	require.NoError(t, store.Save(ctx, entities.Credential{Kind: entities.CredentialSession, Token: "tok-new"}))

	got, err := store.Load(ctx)
	require.NoError(t, err)
	assert.Equal(t, "tok-new", got.Token)
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
