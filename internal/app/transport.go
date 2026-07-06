package app

import (
	"net/http"

	observability "github.com/TraumTech/paas-observability-sdk"
	"github.com/TraumTech/paas-observability-sdk/sdk/observabilityhttp"
)

// bearerTransport прикладывает к каждому исходящему запросу машинный креденшел
// сервиса заголовком `Authorization: Bearer <token>`. Прокси платформы
// (Oathkeeper) валидирует токен introspection'ом и связывает запрос с сервисом —
// сам CLI про схему валидации не знает (auth-агностичные адаптеры).
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

// httpClient собирает клиент платформы. Приоритет у машинного креденшела сервиса
// (CI-сценарий, поведение TKN-06 не меняется); без него запросы идут с токеном
// сессии вошедшего пользователя (`paas-cli auth login`); без того и другого —
// анонимно, как прежде. Observability-транспорт — внешним слоем: спан и
// traceparent появляются до креденшелов, логи и спан покрывают весь запрос.
func httpClient(obs observability.Observer, serviceToken, sessionToken string) *http.Client {
	var base http.RoundTripper
	switch {
	case serviceToken != "":
		base = &bearerTransport{token: serviceToken, base: http.DefaultTransport}
	case sessionToken != "":
		base = &sessionTransport{token: sessionToken, base: http.DefaultTransport}
	}
	return &http.Client{Timeout: httpTimeout, Transport: observabilityhttp.NewTransport(obs, base)}
}
