package app

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

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

	resp, err := httpClient(apitest.NewObserver(), "secret-token", nil).Get(srv.URL)
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

	resp, err := httpClient(apitest.NewObserver(), "", nil).Get(srv.URL)
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

	resp, err := httpClient(apitest.NewObserver(), "", session("session-token")).Get(srv.URL)
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

	resp, err := httpClient(apitest.NewObserver(), "secret-token", session("session-token")).Get(srv.URL)
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

	resp, err := httpClient(apitest.NewObserver(), "", personalToken("paas_uat_secret")).Get(srv.URL)
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
			_, err := httpClient(apitest.NewObserver(), tc.envToken, tc.credential).Get(srv.URL)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, ожидалась %v", err, tc.want)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
