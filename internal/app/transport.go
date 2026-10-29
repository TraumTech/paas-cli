package app

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	observability "github.com/TraumTech/paas-observability-sdk"
	"github.com/TraumTech/paas-observability-sdk/sdk/observabilityhttp"

	"github.com/TraumTech/paas-cli/internal/entities"
	"github.com/TraumTech/paas-cli/internal/usecases"
)

// bearerTransport прикладывает к каждому исходящему запросу токен доступа
// заголовком `Authorization: Bearer <token>` — машинный (токен сервиса) или
// личный токен пользователя (AUTH-19): предъявляются они одинаково. Прокси
// платформы (Oathkeeper) валидирует токен introspection'ом и связывает запрос с
// сервисом либо с человеком — сам CLI про схему валидации не знает
// (auth-агностичные адаптеры).
type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(clone)
}

// sessionTransport прикладывает к каждому исходящему запросу токен сессии
// вошедшего пользователя заголовком `X-Session-Token`. Прокси платформы
// (Oathkeeper) валидирует сессию у identity-провайдера и связывает запрос с
// пользователем — сам CLI про схему валидации не знает (auth-агностичные адаптеры).
type sessionTransport struct {
	token string
	base  http.RoundTripper
}

func (t *sessionTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("X-Session-Token", t.token)
	return t.base.RoundTrip(clone)
}

// unauthorizedTransport переводит отказ платформы «не аутентифицирован» на язык
// пользователя: какой креденшел предъявлялся и, значит, что делать, знает только
// composition root — поэтому перевод живёт здесь, а не в адаптерах (AUTH-17).
type unauthorizedTransport struct {
	base   http.RoundTripper
	reason error
}

func (t *unauthorizedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		return resp, err
	}
	resp.Body.Close()
	return nil, t.reason
}

// httpClient собирает клиент платформы. Приоритет у токена из окружения
// (CI-сценарий, поведение TKN-06 не меняется); без него запросы идут сохранённым
// входом (`paas-cli auth login`) — входом OIDC, который транспорт продлевает
// сам, либо прежним видом: сессией провайдера или личным токеном; без того и
// другого — анонимно, как прежде. Observability-транспорт — внешним слоем: спан
// и traceparent появляются до креденшелов, логи и спан покрывают весь запрос.
func httpClient(obs observability.Observer, envToken string, credential *entities.Credential, refresher loginRefresher, sessions usecases.SessionStore) *http.Client {
	base, reason := http.RoundTripper(http.DefaultTransport), error(entities.ErrLoginRequired)
	switch {
	case envToken != "":
		base = &bearerTransport{token: envToken, base: http.DefaultTransport}
		reason = entities.ErrTokenRejected
	case credential != nil && credential.Kind == entities.CredentialOIDC:
		base = newOIDCTransport(refresher, sessions, *credential, http.DefaultTransport)
		reason = entities.ErrSessionExpired
	case credential != nil && credential.Kind == entities.CredentialPersonalToken:
		base = &bearerTransport{token: credential.Token, base: http.DefaultTransport}
		reason = entities.ErrPersonalTokenRejected
	case credential != nil:
		base = &sessionTransport{token: credential.Token, base: http.DefaultTransport}
		reason = entities.ErrSessionExpired
	}
	unauthorized := &unauthorizedTransport{base: base, reason: reason}
	return &http.Client{Timeout: httpTimeout, Transport: observabilityhttp.NewTransport(obs, unauthorized)}
}

// loginRefresher продлевает вход OIDC по refresh-токену.
type loginRefresher interface {
	Refresh(ctx context.Context, refreshToken string) (*entities.Credential, error)
}

// refreshLeeway — за сколько до истечения токен доступа считается негодным:
// запрос не должен доехать до платформы с токеном, истёкшим в пути.
const refreshLeeway = 30 * time.Second

// oidcTransport прикладывает к каждому запросу токен доступа входа OIDC
// (IAP-03) и сам поддерживает его живым: истекающий токен продлевается до
// запроса, отвергнутый платформой — один раз после, и запрос повторяется.
// Новая пара сохраняется в хранилище входа: refresh-токен ротируется сервером,
// прежний после продления негоден.
type oidcTransport struct {
	refresher  loginRefresher
	sessions   usecases.SessionStore
	base       http.RoundTripper
	now        func() time.Time
	mu         sync.Mutex
	credential entities.Credential
}

func newOIDCTransport(refresher loginRefresher, sessions usecases.SessionStore, credential entities.Credential, base http.RoundTripper) *oidcTransport {
	return &oidcTransport{refresher: refresher, sessions: sessions, base: base, now: time.Now, credential: credential}
}

func (t *oidcTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.accessToken(req.Context(), "")
	if err != nil {
		return nil, err
	}
	resp, err := t.send(req, token)
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		return resp, err
	}
	// Тело без GetBody повторно не прочитать — такой запрос не повторяем.
	if req.Body != nil && req.GetBody == nil {
		return resp, nil
	}
	resp.Body.Close()
	token, err = t.accessToken(req.Context(), token)
	if err != nil {
		return nil, err
	}
	return t.send(req, token)
}

func (t *oidcTransport) send(req *http.Request, token string) (*http.Response, error) {
	clone := req.Clone(req.Context())
	if req.Body != nil && req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		clone.Body = body
	}
	clone.Header.Set("Authorization", "Bearer "+token)
	return t.base.RoundTrip(clone)
}

// accessToken отдаёт живой токен доступа. rejected — токен, который платформа
// только что отвергла: продлеваем, если никто не успел раньше; иначе — если
// срок ещё не вышел, токен годен как есть.
func (t *oidcTransport) accessToken(ctx context.Context, rejected string) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	stale := t.credential.Token == rejected || t.credential.ExpiresAt.Before(t.now().Add(refreshLeeway))
	if !stale {
		return t.credential.Token, nil
	}
	fresh, err := t.refresher.Refresh(ctx, t.credential.RefreshToken)
	if err != nil {
		return "", err
	}
	// Что нельзя получить от сервера — e-mail — переносим из прежнего входа.
	if fresh.Email == "" {
		fresh.Email = t.credential.Email
	}
	if err := t.sessions.Save(ctx, *fresh); err != nil {
		return "", fmt.Errorf("сохранение продлённого входа: %w", err)
	}
	t.credential = *fresh
	return t.credential.Token, nil
}
