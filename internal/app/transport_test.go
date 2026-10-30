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

// Секрет прежнего вида (до IAP-04) не предъявляется: платформа его не примет,
// а отказ объясняет, что токен нужно перевыпустить.
func TestHTTPClientDoesNotSendLegacyToken(t *testing.T) {
	var present bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header["Authorization"]
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := httpClient(apitest.NewObserver(), "paas_pat_legacy", nil, nil, nil).Get(srv.URL)

	if !errors.Is(err, entities.ErrLegacyToken) {
		t.Fatalf("err = %v, ожидалась %v", err, entities.ErrLegacyToken)
	}
	if present {
		t.Fatal("секрет прежнего вида не должен отправляться")
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
		{name: "токен прежнего вида — просим перевыпустить", envToken: "secret-token", want: entities.ErrLegacyToken},
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

// Токен платформы (IAP-04): каждый грант — новый JWT.
func (r *refresherStub) ClientCredentials(_ context.Context, token entities.MachineToken) (*entities.Credential, error) {
	n := r.calls.Add(1)
	if r.err != nil {
		return nil, r.err
	}
	return &entities.Credential{Kind: entities.CredentialMachine, Token: "jwt-" + token.ClientID + "-" + string(rune('0'+n)), ExpiresAt: time.Now().Add(10 * time.Minute)}, nil
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

// Токен платформы нового вида не уходит в заголовке сам: первый запрос меняет
// его на JWT (client_credentials), второй идёт с тем же JWT.
func TestMachineTransport_ExchangesTokenForJWT(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = append(got, r.Header.Get("Authorization")) }))
	defer srv.Close()
	grantor := &refresherStub{}
	client := httpClient(apitest.NewObserver(), "paas_pat_id.s3cret", nil, grantor, &storeStub{})

	for range 2 {
		resp, err := client.Get(srv.URL)
		if err != nil {
			t.Fatalf("запрос не удался: %v", err)
		}
		resp.Body.Close()
	}

	if len(got) != 2 || got[0] != "Bearer jwt-paas_pat_id-1" || got[1] != "Bearer jwt-paas_pat_id-1" {
		t.Fatalf("Authorization = %q, ожидался один и тот же JWT", got)
	}
	if grantor.calls.Load() != 1 {
		t.Fatalf("грантов %d, ожидался один на процесс", grantor.calls.Load())
	}
}

// Отвергнутый токен платформы (отозван) — понятная ошибка, а не 401.
func TestMachineTransport_RejectedToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	_, err := httpClient(apitest.NewObserver(), "paas_pat_id.s3cret", nil, &refresherStub{err: entities.ErrTokenRejected}, &storeStub{}).Get(srv.URL)

	if !errors.Is(err, entities.ErrTokenRejected) {
		t.Fatalf("err = %v, ожидалась ErrTokenRejected", err)
	}
}
