package entities

import "time"

// UserSession — кому принадлежит вход: e-mail владельца для отчёта «под кем
// вошёл» и его id у платформы (на случай, если e-mail вход не несёт).
type UserSession struct {
	Email  string
	UserID string
}

// CredentialKind — чем подтверждён вход в CLI. Вход через OIDC (IAP-03) — пара
// «токен доступа + refresh» от сервера авторизации платформы: короткий JWT
// предъявляется заголовком Authorization, а продлевается refresh-токеном без
// участия человека. Прежние виды (сессия провайдера, личный токен) платформа с
// IAP-05 не принимает — такой сохранённый вход считается отсутствующим.
type CredentialKind string

const (
	CredentialOIDC CredentialKind = "oidc"
	// CredentialMachine — JWT, полученный за токен платформы из PAAS_API_TOKEN
	// (IAP-04): живёт только в процессе, в файл входа не пишется.
	CredentialMachine CredentialKind = "machine"
)

// Credential — сохранённый локально вход. Помимо токена доступа хранит то, что
// нельзя восстановить: вид креденшела, refresh-токен (им продлевается вход
// OIDC), e-mail — под кем вошли, и срок действия токена доступа.
type Credential struct {
	Kind         CredentialKind `json:"kind"`
	Token        string         `json:"token"`
	RefreshToken string         `json:"refresh_token,omitempty"`
	Email        string         `json:"email,omitempty"`
	ExpiresAt    time.Time      `json:"expires_at,omitempty"`
}
