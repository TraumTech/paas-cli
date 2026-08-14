package app

import (
	"net/http"

	observability "github.com/TraumTech/paas-observability-sdk"
	"github.com/TraumTech/paas-observability-sdk/sdk/observabilityhttp"

	"github.com/TraumTech/paas-cli/internal/entities"
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
// входом (`paas-cli auth login`) — сессией провайдера или личным токеном; без
// того и другого — анонимно, как прежде. Observability-транспорт — внешним
// слоем: спан и traceparent появляются до креденшелов, логи и спан покрывают
// весь запрос.
func httpClient(obs observability.Observer, envToken string, credential *entities.Credential) *http.Client {
	base, reason := http.RoundTripper(http.DefaultTransport), error(entities.ErrLoginRequired)
	switch {
	case envToken != "":
		base = &bearerTransport{token: envToken, base: http.DefaultTransport}
		reason = entities.ErrTokenRejected
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
