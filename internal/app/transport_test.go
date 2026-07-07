package app

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TraumTech/paas-observability-sdk/apitest"

	"github.com/TraumTech/paas-cli/internal/entities"
)

func TestHTTPClientSendsBearerTokenWhenSet(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
	}))
	defer srv.Close()

	resp, err := httpClient(apitest.NewObserver(), "secret-token", "").Get(srv.URL)
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

	resp, err := httpClient(apitest.NewObserver(), "", "").Get(srv.URL)
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

	resp, err := httpClient(apitest.NewObserver(), "", "session-token").Get(srv.URL)
	if err != nil {
		t.Fatalf("запрос не удался: %v", err)
	}
	resp.Body.Close()

	if got != "session-token" {
		t.Fatalf("X-Session-Token = %q, ожидался %q", got, "session-token")
	}
}

func TestHTTPClientServiceTokenTakesPrecedenceOverSession(t *testing.T) {
	var auth, session string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		session = r.Header.Get("X-Session-Token")
	}))
	defer srv.Close()

	resp, err := httpClient(apitest.NewObserver(), "secret-token", "session-token").Get(srv.URL)
	if err != nil {
		t.Fatalf("запрос не удался: %v", err)
	}
	resp.Body.Close()

	if auth != "Bearer secret-token" {
		t.Fatalf("Authorization = %q, ожидался машинный токен", auth)
	}
	if session != "" {
		t.Fatal("при машинном токене сессия пользователя не должна отправляться")
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
		name         string
		serviceToken string
		sessionToken string
		want         error
	}{
		{name: "аноним — предлагаем войти или задать токен", want: entities.ErrLoginRequired},
		{name: "сессия — предлагаем войти заново", sessionToken: "session-token", want: entities.ErrSessionExpired},
		{name: "токен сервиса — сообщаем, что не принят", serviceToken: "secret-token", want: entities.ErrServiceTokenRejected},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := httpClient(apitest.NewObserver(), tc.serviceToken, tc.sessionToken).Get(srv.URL)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, ожидалась %v", err, tc.want)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
