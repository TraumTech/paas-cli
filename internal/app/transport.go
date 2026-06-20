package app

import "net/http"

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

// httpClient собирает клиент платформы. Если задан токен сервиса, запросы
// уходят с ним; без токена клиент обращается к платформе как прежде.
func httpClient(token string) *http.Client {
	client := &http.Client{Timeout: httpTimeout}
	if token != "" {
		client.Transport = &bearerTransport{token: token, base: http.DefaultTransport}
	}
	return client
}
