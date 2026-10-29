package browserauthorizerlocal

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// flowStub изображает сервер авторизации: собирает ссылку входа и помнит, что
// CLI принёс на обмен.
type flowStub struct {
	exchanged struct{ code, verifier, redirectURI string }
	challenge string
	err       error
}

func (f *flowStub) AuthorizeURL(redirectURI, state, codeChallenge string) string {
	f.challenge = codeChallenge
	return "https://oauth.example/oauth2/auth?" + url.Values{
		"redirect_uri": {redirectURI}, "state": {state}, "code_challenge": {codeChallenge},
	}.Encode()
}

func (f *flowStub) Exchange(_ context.Context, code, verifier, redirectURI string) (*entities.Credential, error) {
	f.exchanged.code, f.exchanged.verifier, f.exchanged.redirectURI = code, verifier, redirectURI
	if f.err != nil {
		return nil, f.err
	}
	return &entities.Credential{Kind: entities.CredentialOIDC, Token: "access", RefreshToken: "refresh", Email: "user@example.com"}, nil
}

// browserStub изображает браузер: разбирает ссылку входа и стучится на адрес
// возврата с ответом сервера авторизации.
func browserStub(t *testing.T, respond func(callback string, state string)) func(string) error {
	t.Helper()
	return func(target string) error {
		parsed, err := url.Parse(target)
		require.NoError(t, err)
		query := parsed.Query()
		go respond(query.Get("redirect_uri"), query.Get("state"))
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

func TestAuthorize_ExchangesCodeWithPKCE(t *testing.T) {
	flow := &flowStub{}
	authorizer := New(flow, io.Discard)
	var redirectURI string
	authorizer.openBrowser = browserStub(t, func(callback, state string) {
		redirectURI = callback
		get(t, callback+"?state="+state+"&code=code-1")
	})

	credential, err := authorizer.Authorize(context.Background())

	require.NoError(t, err)
	assert.Equal(t, entities.CredentialOIDC, credential.Kind)
	assert.Equal(t, "user@example.com", credential.Email)
	assert.Equal(t, "code-1", flow.exchanged.code)
	assert.Equal(t, redirectURI, flow.exchanged.redirectURI)
	sum := sha256.Sum256([]byte(flow.exchanged.verifier))
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(sum[:]), flow.challenge, "challenge — SHA-256 от verifier")
	assert.GreaterOrEqual(t, len(flow.exchanged.verifier), 43)
}

// Пользователь отказал в браузере — вход не выполняется, и CLI не ждёт таймаута.
func TestAuthorize_Denied(t *testing.T) {
	authorizer := New(&flowStub{}, io.Discard)
	authorizer.openBrowser = browserStub(t, func(callback, state string) {
		get(t, callback+"?state="+state+"&error=access_denied")
	})

	_, err := authorizer.Authorize(context.Background())

	assert.ErrorIs(t, err, entities.ErrAuthorizationDenied)
}

// Иной отказ сервера авторизации доносится с его кодом.
func TestAuthorize_ServerError(t *testing.T) {
	authorizer := New(&flowStub{}, io.Discard)
	authorizer.openBrowser = browserStub(t, func(callback, state string) {
		get(t, callback+"?state="+state+"&error=invalid_scope&error_description=nope")
	})

	_, err := authorizer.Authorize(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_scope")
}

// Чужой ответ (другой запуск CLI, подложенная ссылка) не принимается: свой
// ответ узнаётся по одноразовому state.
func TestAuthorize_ForeignStateIgnored(t *testing.T) {
	flow := &flowStub{}
	authorizer := New(flow, io.Discard)
	authorizer.openBrowser = browserStub(t, func(callback, state string) {
		get(t, callback+"?state=someone-else&code=stolen")
		get(t, callback+"?state="+state+"&code=mine")
	})

	_, err := authorizer.Authorize(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "mine", flow.exchanged.code)
}

// Браузер открыть не удалось — ссылка уже напечатана, ждём, что её откроют руками.
func TestAuthorize_BrowserUnavailable_KeepsWaiting(t *testing.T) {
	authorizer := New(&flowStub{}, io.Discard)
	authorizer.openBrowser = func(string) error { return assert.AnError }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := authorizer.Authorize(ctx)

	assert.ErrorIs(t, err, context.Canceled)
}

// Ссылка входа ведёт к серверу авторизации и несёт петлевой адрес возврата.
func TestAuthorize_RedirectURIIsLoopback(t *testing.T) {
	flow := &flowStub{}
	authorizer := New(flow, io.Discard)
	var got *url.URL
	authorizer.openBrowser = func(target string) error {
		parsed, err := url.Parse(target)
		require.NoError(t, err)
		got = parsed
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _ = authorizer.Authorize(ctx)

	require.NotNil(t, got)
	callback, err := url.Parse(got.Query().Get("redirect_uri"))
	require.NoError(t, err)
	assert.Equal(t, "http", callback.Scheme)
	assert.Equal(t, "127.0.0.1", callback.Hostname())
	assert.Equal(t, callbackPath, callback.Path)
	assert.NotEmpty(t, got.Query().Get("state"))
}
