package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TraumTech/paas-observability-sdk/apitest"

	"github.com/TraumTech/paas-cli/internal/entities"
)

// session — сохранённый вход сессией провайдера; личный токен предъявляется
// иначе (см. отдельный тест ниже).
func session(token string) *entities.Credential {
	return &entities.Credential{Kind: entities.CredentialSession, Token: token}
}

func personalToken(token string) *entities.Credential {
	return &entities.Credential{Kind: entities.CredentialPersonalToken, Token: token}
}

func TestHTTPClientSendsBearerTokenWhenSet(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
	}))
	defer srv.Close()

	resp, err := httpClient(apitest.NewObserver(), "secret-token", nil, nil, nil).Get(srv.URL)
	if err != nil {
		t.Fatalf("запрос не удался: %v", err)
	}
	resp.Body.Close()

	if want := "Bearer secret-token"; got != want {
		t.Fatalf("Authorization = %q, ожидался %q", got, want)
	}
}

func TestHTTPClientOmitsAuthorizationWhenTokenEmpty(t *testing.T) {
	var present bool
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, present = r.Header["Authorization"]
	}))
	defer srv.Close()

	resp, err := httpClient(apitest.NewObserver(), "", nil, nil, nil).Get(srv.URL)
	if err != nil {
		t.Fatalf("запрос не удался: %v", err)
	}
	resp.Body.Close()

	if present {
		t.Fatal("без токена заголовок Authorization не должен отправляться")
	}
}

func TestHTTPClientSendsSessionTokenWhenNoServiceToken(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Session-Token")
	}))
	defer srv.Close()

	resp, err := httpClient(apitest.NewObserver(), "", session("session-token"), nil, nil).Get(srv.URL)
	if err != nil {
		t.Fatalf("запрос не удался: %v", err)
	}
	resp.Body.Close()

	if got != "session-token" {
		t.Fatalf("X-Session-Token = %q, ожидался %q", got, "session-token")
	}
}

func TestHTTPClientServiceTokenTakesPrecedenceOverSession(t *testing.T) {
	var auth, sessionHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		sessionHeader = r.Header.Get("X-Session-Token")
	}))
	defer srv.Close()

	resp, err := httpClient(apitest.NewObserver(), "secret-token", session("session-token"), nil, nil).Get(srv.URL)
	if err != nil {
		t.Fatalf("запрос не удался: %v", err)
	}
	resp.Body.Close()

	if auth != "Bearer secret-token" {
		t.Fatalf("Authorization = %q, ожидался машинный токен", auth)
	}
	if sessionHeader != "" {
		t.Fatal("при машинном токене сессия пользователя не должна отправляться")
	}
}

// Личный токен предъявляется как машинный — заголовком Authorization: за ним
// стоит человек, но способ предъявления у токенов доступа один.
func TestHTTPClientSendsPersonalTokenAsBearer(t *testing.T) {
	var auth, sessionHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		sessionHeader = r.Header.Get("X-Session-Token")
	}))
	defer srv.Close()

	resp, err := httpClient(apitest.NewObserver(), "", personalToken("paas_uat_secret"), nil, nil).Get(srv.URL)
	if err != nil {
		t.Fatalf("запрос не удался: %v", err)
	}
	resp.Body.Close()

	if auth != "Bearer paas_uat_secret" {
		t.Fatalf("Authorization = %q, ожидался личный токен", auth)
	}
	if sessionHeader != "" {
		t.Fatal("личный токен не предъявляется как сессия провайдера")
	}
}

func TestBearerTransportDoesNotMutateOriginalRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.test", nil)
	transport := &bearerTransport{token: "tok", base: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
	})}

	if _, err := transport.RoundTrip(req); err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("исходный запрос мутировал: Authorization = %q", got)
	}
}

func TestHTTPClientTranslatesUnauthorizedByCredential(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	cases := []struct {
		name       string
		envToken   string
		credential *entities.Credential
		want       error
	}{
		{name: "аноним — предлагаем войти или задать токен", want: entities.ErrLoginRequired},
		{name: "сессия — предлагаем войти заново", credential: session("session-token"), want: entities.ErrSessionExpired},
		{name: "токен из окружения — сообщаем, что не принят", envToken: "secret-token", want: entities.ErrTokenRejected},
		{name: "личный токен — истёк или отозван", credential: personalToken("paas_uat_secret"), want: entities.ErrPersonalTokenRejected},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := httpClient(apitest.NewObserver(), tc.envToken, tc.credential, nil, nil).Get(srv.URL)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, ожидалась %v", err, tc.want)
			}
		})
	}
}

func oidc(access, refresh string, expiresAt time.Time) *entities.Credential {
	return &entities.Credential{Kind: entities.CredentialOIDC, Token: access, RefreshToken: refresh, ExpiresAt: expiresAt, Email: "user@example.com"}
}

// refresherStub — сервер авторизации: каждое продление выдаёт следующую пару.
type refresherStub struct {
	calls atomic.Int32
	err   error
}

func (r *refresherStub) Refresh(_ context.Context, refresh string) (*entities.Credential, error) {
	n := r.calls.Add(1)
	if r.err != nil {
		return nil, r.err
	}
	return &entities.Credential{Kind: entities.CredentialOIDC, Token: "access-" + refresh + "-" + string(rune('0'+n)), RefreshToken: "refresh-" + string(rune('0'+n)), ExpiresAt: time.Now().Add(10 * time.Minute)}, nil
}

type storeStub struct {
	saved []entities.Credential
}

func (s *storeStub) Save(_ context.Context, c entities.Credential) error {
	s.saved = append(s.saved, c)
	return nil
}
func (s *storeStub) Load(context.Context) (*entities.Credential, error) {
	return nil, entities.ErrNoSession
}
func (s *storeStub) Delete(context.Context) error { return nil }

func TestOIDCTransport_SendsLiveAccessToken(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = r.Header.Get("Authorization") }))
	defer srv.Close()
	refresher, store := &refresherStub{}, &storeStub{}

	resp, err := httpClient(apitest.NewObserver(), "", oidc("access-1", "refresh-1", time.Now().Add(time.Hour)), refresher, store).Get(srv.URL)
	if err != nil {
		t.Fatalf("запрос не удался: %v", err)
	}
	resp.Body.Close()

	if got != "Bearer access-1" {
		t.Fatalf("Authorization = %q, ожидался живой токен доступа", got)
	}
	if refresher.calls.Load() != 0 || len(store.saved) != 0 {
		t.Fatal("живой токен продлевать не нужно")
	}
}

// Истекающий токен продлевается до запроса, и новая пара сохраняется: прежний
// refresh-токен после ротации негоден.
func TestOIDCTransport_RefreshesExpiringTokenBeforeRequest(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = r.Header.Get("Authorization") }))
	defer srv.Close()
	refresher, store := &refresherStub{}, &storeStub{}

	resp, err := httpClient(apitest.NewObserver(), "", oidc("access-old", "refresh-1", time.Now().Add(5*time.Second)), refresher, store).Get(srv.URL)
	if err != nil {
		t.Fatalf("запрос не удался: %v", err)
	}
	resp.Body.Close()

	if got != "Bearer access-refresh-1-1" {
		t.Fatalf("Authorization = %q, ожидался продлённый токен", got)
	}
	if len(store.saved) != 1 || store.saved[0].RefreshToken != "refresh-1" {
		t.Fatalf("продлённый вход не сохранён: %+v", store.saved)
	}
	if store.saved[0].Email != "user@example.com" {
		t.Fatal("e-mail прежнего входа должен переноситься в продлённый")
	}
}

// Отвергнутый платформой токен (отозван до срока) продлевается один раз, и
// запрос — вместе с телом — повторяется.
func TestOIDCTransport_RetriesOnceAfterUnauthorized(t *testing.T) {
	var auths []string
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		auths = append(auths, r.Header.Get("Authorization"))
		bodies = append(bodies, string(body))
		if len(auths) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer srv.Close()
	refresher, store := &refresherStub{}, &storeStub{}

	client := httpClient(apitest.NewObserver(), "", oidc("access-1", "refresh-1", time.Now().Add(time.Hour)), refresher, store)
	resp, err := client.Post(srv.URL, "text/plain", strings.NewReader("payload"))
	if err != nil {
		t.Fatalf("запрос не удался: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK || len(auths) != 2 {
		t.Fatalf("ожидался повтор запроса после 401: статус %d, запросов %d", resp.StatusCode, len(auths))
	}
	if auths[1] != "Bearer access-refresh-1-1" || bodies[1] != "payload" {
		t.Fatalf("повтор должен идти новым токеном и с тем же телом: %v %v", auths, bodies)
	}
}

// Продлить нечем (refresh истёк или отозван) — пользователю предлагается войти
// заново, а не повторный 401.
func TestOIDCTransport_RefreshFailureAsksToLogin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }))
	defer srv.Close()
	refresher := &refresherStub{err: entities.ErrSessionExpired}

	_, err := httpClient(apitest.NewObserver(), "", oidc("access-1", "refresh-1", time.Now().Add(time.Hour)), refresher, &storeStub{}).Get(srv.URL)

	if !errors.Is(err, entities.ErrSessionExpired) {
		t.Fatalf("err = %v, ожидалась %v", err, entities.ErrSessionExpired)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
