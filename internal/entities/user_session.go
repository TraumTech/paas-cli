package entities

import "time"

// UserSession — кому принадлежит вход: e-mail владельца для отчёта «под кем
// вошёл» и его id у платформы (на случай, если e-mail вход не несёт); у сессии
// провайдера — ещё и её токен.
type UserSession struct {
	Token  string
	Email  string
	UserID string
}

// CredentialKind — чем подтверждён вход в CLI. Вход через OIDC (IAP-03) — пара
// «токен доступа + refresh» от сервера авторизации платформы: короткий JWT
// предъявляется заголовком Authorization, а продлевается refresh-токеном без
// участия человека. Прежние виды — сессия провайдера (проверяется у него) и
// личный токен платформы (AUTH-22) — читаются, пока платформа их принимает.
type CredentialKind string

const (
	CredentialOIDC          CredentialKind = "oidc"
	CredentialSession       CredentialKind = "session"
	CredentialPersonalToken CredentialKind = "personal_token"
)

// Credential — сохранённый локально вход. Помимо секрета хранит то, что нельзя
// восстановить: вид креденшела (чем его предъявлять и где проверять), refresh-
// токен (им продлевается вход OIDC), id токена (по нему выход отзывает личный
// токен на платформе), e-mail — под кем вошли, и срок действия секрета: у входа
// OIDC — токена доступа, у личного токена — его самого.
type Credential struct {
	Kind         CredentialKind `json:"kind"`
	Token        string         `json:"token"`
	RefreshToken string         `json:"refresh_token,omitempty"`
	TokenID      string         `json:"token_id,omitempty"`
	Email        string         `json:"email,omitempty"`
	ExpiresAt    time.Time      `json:"expires_at,omitempty"`
}

// PersonalToken — личный токен пользователя на платформе (метаданные, без
// секрета): им CLI подтверждает, что сохранённый вход ещё жив.
type PersonalToken struct {
	ID        string
	Name      string
	ExpiresAt time.Time
}
