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

	require.NoError(t, store.Save(ctx, entities.Credential{Kind: entities.CredentialOIDC, Token: "tok-1"}))
	_, err := store.Load(ctx)
	require.NoError(t, err)

	require.NoError(t, store.Delete(ctx))
	_, err = store.Load(ctx)
	assert.ErrorIs(t, err, entities.ErrNoSession)
}

// Вход OIDC (IAP-03) хранит пару токенов и срок токена доступа.
func TestStore_SaveLoad_OIDC(t *testing.T) {
	withTempConfigDir(t)
	store := New()
	ctx := context.Background()
	expiresAt := time.Date(2026, 10, 29, 12, 10, 0, 0, time.UTC)

	require.NoError(t, store.Save(ctx, entities.Credential{
		Kind:         entities.CredentialOIDC,
		Token:        "access-1",
		RefreshToken: "refresh-1",
		Email:        "user@example.com",
		ExpiresAt:    expiresAt,
	}))

	got, err := store.Load(ctx)
	require.NoError(t, err)
	assert.Equal(t, entities.CredentialOIDC, got.Kind)
	assert.Equal(t, "access-1", got.Token)
	assert.Equal(t, "refresh-1", got.RefreshToken)
	assert.Equal(t, "user@example.com", got.Email)
	assert.True(t, expiresAt.Equal(got.ExpiresAt))
}

// Вход прежних версий CLI — голый токен сессии или сессия/личный токен в JSON —
// платформа не принимает (IAP-05): он читается как отсутствующий, и CLI просит
// войти заново.
func TestStore_Load_LegacyCredentialIsNoSession(t *testing.T) {
	for name, content := range map[string]string{
		"голый токен сессии": "ory_st_legacy\n",
		"сессия":             `{"kind":"session","token":"ory_st_legacy"}`,
		"без вида":           `{"token":"ory_st_legacy"}`,
		"личный токен":       `{"kind":"personal_token","token":"paas_uat_secret","token_id":"t-1"}`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := withTempConfigDir(t)
			require.NoError(t, os.MkdirAll(filepath.Join(dir, "paas-cli"), 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "paas-cli", "session"), []byte(content), 0o600))

			_, err := New().Load(context.Background())

			assert.ErrorIs(t, err, entities.ErrNoSession)
		})
	}
}

func TestStore_Save_OwnerOnlyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("права POSIX не проверяются на windows")
	}
	dir := withTempConfigDir(t)
	store := New()

	require.NoError(t, store.Save(context.Background(), entities.Credential{
		Kind: entities.CredentialOIDC, Token: "tok-1",
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

	require.NoError(t, store.Save(ctx, entities.Credential{Kind: entities.CredentialOIDC, Token: "tok-old"}))
	require.NoError(t, store.Save(ctx, entities.Credential{Kind: entities.CredentialOIDC, Token: "tok-new"}))

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
