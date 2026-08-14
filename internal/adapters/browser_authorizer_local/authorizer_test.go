package browserauthorizerlocal

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// browserStub изображает браузер: разбирает ссылку подтверждения и стучится на
// адрес возврата с готовым ответом платформы.
func browserStub(t *testing.T, respond func(callback string, state string)) func(string) error {
	t.Helper()
	return func(target string) error {
		parsed, err := url.Parse(target)
		require.NoError(t, err)
		query := parsed.Query()
		go respond(query.Get("callback"), query.Get("state"))
		return nil
	}
}

func get(t *testing.T, target string) {
	t.Helper()
	resp, err := http.Get(target)
	if err != nil {
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

func TestAuthorize_ReturnsIssuedToken(t *testing.T) {
	authorizer := New("https://paas.example", io.Discard)
	expiresAt := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	authorizer.openBrowser = browserStub(t, func(callback, state string) {
		query := url.Values{}
		query.Set("state", state)
		query.Set("token", "paas_uat_secret")
		query.Set("token_id", "tok-id")
		query.Set("email", "user@example.com")
		query.Set("expires_at", expiresAt.Format(time.RFC3339))
		get(t, callback+"?"+query.Encode())
	})

	credential, err := authorizer.Authorize(context.Background())

	require.NoError(t, err)
	assert.Equal(t, entities.CredentialPersonalToken, credential.Kind)
	assert.Equal(t, "paas_uat_secret", credential.Token)
	assert.Equal(t, "tok-id", credential.TokenID)
	assert.Equal(t, "user@example.com", credential.Email)
	assert.True(t, expiresAt.Equal(credential.ExpiresAt))
}

// Пользователь отказал в браузере — вход не выполняется, и CLI не ждёт таймаута.
func TestAuthorize_Denied(t *testing.T) {
	authorizer := New("https://paas.example", io.Discard)
	authorizer.openBrowser = browserStub(t, func(callback, state string) {
		get(t, callback+"?state="+state+"&error=denied")
	})

	_, err := authorizer.Authorize(context.Background())

	assert.ErrorIs(t, err, entities.ErrAuthorizationDenied)
}

// Чужой ответ (другой запуск CLI, подложенная ссылка) не принимается: свой
// ответ узнаётся по одноразовому нонсу.
func TestAuthorize_ForeignStateIgnored(t *testing.T) {
	authorizer := New("https://paas.example", io.Discard)
	authorizer.openBrowser = browserStub(t, func(callback, state string) {
		get(t, callback+"?state=someone-else&token=paas_uat_stolen")
		// Свой ответ приходит следом — вход завершается им.
		get(t, callback+"?state="+state+"&token=paas_uat_secret&token_id=tok-id")
	})

	credential, err := authorizer.Authorize(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "paas_uat_secret", credential.Token)
}

// Браузер открыть не удалось — подсказываем путь без него, а не молча ждём.
func TestAuthorize_BrowserUnavailable(t *testing.T) {
	authorizer := New("https://paas.example", io.Discard)
	authorizer.openBrowser = func(string) error { return assert.AnError }

	_, err := authorizer.Authorize(context.Background())

	assert.ErrorIs(t, err, entities.ErrBrowserUnavailable)
}

// Ссылка подтверждения ведёт в интерфейс платформы и несёт петлевой адрес
// возврата, нонс и имя машины — по нему человек узнаёт, кто просит доступ.
func TestAuthorize_AuthorizeURLShape(t *testing.T) {
	authorizer := New("https://paas.example/", io.Discard)
	var got *url.URL
	authorizer.openBrowser = func(target string) error {
		parsed, err := url.Parse(target)
		require.NoError(t, err)
		got = parsed
		return assert.AnError // дальше идти незачем: проверяем только ссылку
	}

	_, _ = authorizer.Authorize(context.Background())

	require.NotNil(t, got)
	assert.Equal(t, "https://paas.example/cli/authorize", got.Scheme+"://"+got.Host+got.Path)
	callback, err := url.Parse(got.Query().Get("callback"))
	require.NoError(t, err)
	assert.Equal(t, "http", callback.Scheme)
	assert.Equal(t, "127.0.0.1", callback.Hostname())
	assert.Equal(t, callbackPath, callback.Path)
	assert.NotEmpty(t, got.Query().Get("state"))
	assert.NotEmpty(t, got.Query().Get("label"))
}
