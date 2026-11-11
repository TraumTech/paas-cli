package app

import "net/http"

const (
	// envOrganizationID — организация, в которой работает CLI (TRA-506). Выбор
	// у каждого клиента свой: интерфейс и агент от него не переключаются.
	// Пусто — личная организация пользователя.
	envOrganizationID  = "PAAS_ORGANIZATION_ID"
	organizationHeader = "X-Organization-Id"
)

// organizationTransport передаёт выбранную организацию в каждом запросе.
// Токенам сервисов и CI платформа заголовок не учитывает: их организация своя.
type organizationTransport struct {
	base           http.RoundTripper
	organizationID string
}

func (t *organizationTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set(organizationHeader, t.organizationID)
	return t.base.RoundTrip(clone)
}

// withOrganization оборачивает клиент платформы выбором организации, если он задан.
func withOrganization(client *http.Client, organizationID string) *http.Client {
	if organizationID == "" {
		return client
	}
	client.Transport = &organizationTransport{base: client.Transport, organizationID: organizationID}
	return client
}
