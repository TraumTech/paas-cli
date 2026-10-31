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
// (CI-сценарий, поведение TKN-06 не меняется): токен платформы
// (`<клиент>.<секрет>`, IAP-04) транспорт сам меняет на короткий JWT и
// обновляет его; секрет прежнего вида платформа не принимает (IAP-05) — он не
// предъявляется, а отказ объясняет, что токен нужно перевыпустить. Без токена в
// GitHub Actions с правом `id-token: write` предъявляется токен запуска
// (TRA-485): платформа узнаёт сервис по репозиторию. Иначе запросы идут
// сохранённым входом OIDC (`paas-cli auth login`), который
// транспорт продлевает сам; без того и другого — анонимно. Observability-транспорт — внешним слоем: спан и traceparent
// появляются до креденшелов, логи и спан покрывают весь запрос.
func httpClient(obs observability.Observer, envToken string, credential *entities.Credential, oauth tokenGrantor, sessions usecases.SessionStore, ci ciTokenSource) *http.Client {
	base, reason := http.RoundTripper(http.DefaultTransport), error(entities.ErrLoginRequired)
	switch {
	case envToken != "":
		reason = entities.ErrLegacyToken
		if machine, ok := entities.ParseMachineToken(envToken); ok {
			base = newMachineTransport(oauth, machine, http.DefaultTransport)
			reason = entities.ErrTokenRejected
		}
	case ci != nil:
		base = newCITransport(ci, http.DefaultTransport)
		reason = entities.ErrCITokenRejected
	case credential != nil:
		base = newOIDCTransport(oauth, sessions, *credential, http.DefaultTransport)
		reason = entities.ErrSessionExpired
	}
	unauthorized := &unauthorizedTransport{base: base, reason: reason}
	return &http.Client{Timeout: httpTimeout, Transport: observabilityhttp.NewTransport(obs, unauthorized)}
}

// tokenGrantor — сервер авторизации платформы: продлевает вход OIDC по
// refresh-токену и выдаёт JWT за токен платформы (IAP-04).
type tokenGrantor interface {
	Refresh(ctx context.Context, refreshToken string) (*entities.Credential, error)
	ClientCredentials(ctx context.Context, token entities.MachineToken) (*entities.Credential, error)
}

// ciTokenSource — токен запуска CI (TRA-485): каждый вызов — свежий короткий JWT
// от хостинга, обмена на стороне платформы нет.
type ciTokenSource interface {
	Token(ctx context.Context) (*entities.Credential, error)
}

// refreshLeeway — за сколько до истечения токен доступа считается негодным:
// запрос не должен доехать до платформы с токеном, истёкшим в пути.
const refreshLeeway = 30 * time.Second

// accessTokenTransport прикладывает к каждому запросу короткий токен доступа и
// сам поддерживает его живым: истекающий обновляется до запроса, отвергнутый
// платформой — один раз после, и запрос повторяется. Как именно добывается
// новый токен, знает renew: у входа OIDC (IAP-03) — refresh-токеном с
// сохранением новой пары (refresh ротируется сервером, прежний негоден), у
// токена платформы (IAP-04) — повторным client_credentials.
type accessTokenTransport struct {
	renew      func(ctx context.Context, current entities.Credential) (*entities.Credential, error)
	base       http.RoundTripper
	now        func() time.Time
	mu         sync.Mutex
	credential entities.Credential
}

func newOIDCTransport(refresher tokenGrantor, sessions usecases.SessionStore, credential entities.Credential, base http.RoundTripper) *accessTokenTransport {
	renew := func(ctx context.Context, current entities.Credential) (*entities.Credential, error) {
		fresh, err := refresher.Refresh(ctx, current.RefreshToken)
		if err != nil {
			return nil, err
		}
		// Что нельзя получить от сервера — e-mail — переносим из прежнего входа.
		if fresh.Email == "" {
			fresh.Email = current.Email
		}
		if err := sessions.Save(ctx, *fresh); err != nil {
			return nil, fmt.Errorf("сохранение продлённого входа: %w", err)
		}
		return fresh, nil
	}
	return &accessTokenTransport{renew: renew, base: base, now: time.Now, credential: credential}
}

// newMachineTransport: JWT ещё нет — первый запрос его и добудет (пустой
// креденшел с нулевым сроком — заведомо устаревший).
func newMachineTransport(grantor tokenGrantor, token entities.MachineToken, base http.RoundTripper) *accessTokenTransport {
	renew := func(ctx context.Context, _ entities.Credential) (*entities.Credential, error) {
		return grantor.ClientCredentials(ctx, token)
	}
	return &accessTokenTransport{renew: renew, base: base, now: time.Now}
}

// newCITransport: токен запуска CI запрашивается у хостинга при первом запросе
// и заново, когда истекает.
func newCITransport(source ciTokenSource, base http.RoundTripper) *accessTokenTransport {
	renew := func(ctx context.Context, _ entities.Credential) (*entities.Credential, error) {
		return source.Token(ctx)
	}
	return &accessTokenTransport{renew: renew, base: base, now: time.Now}
}

func (t *accessTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
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

func (t *accessTokenTransport) send(req *http.Request, token string) (*http.Response, error) {
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
// только что отвергла: обновляем, если никто не успел раньше; иначе — если
// срок ещё не вышел, токен годен как есть.
func (t *accessTokenTransport) accessToken(ctx context.Context, rejected string) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	stale := t.credential.Token == rejected || t.credential.ExpiresAt.Before(t.now().Add(refreshLeeway))
	if !stale {
		return t.credential.Token, nil
	}
	fresh, err := t.renew(ctx, t.credential)
	if err != nil {
		return "", err
	}
	t.credential = *fresh
	return t.credential.Token, nil
}
