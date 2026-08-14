package entities

import "time"

// UserSession — сессия вошедшего пользователя у identity-провайдера: токен для
// предъявления платформе и e-mail владельца для отчёта «под кем вошёл».
type UserSession struct {
	Token string
	Email string
}

// CredentialKind — чем подтверждён вход в CLI. Сессия провайдера живёт у него и
// проверяется там же; личный токен (AUTH-22) выпускает платформа и предъявляется
// ей как машинный — заголовком Authorization, но действует от имени человека.
type CredentialKind string

const (
	CredentialSession       CredentialKind = "session"
	CredentialPersonalToken CredentialKind = "personal_token"
)

// Credential — сохранённый локально вход. Помимо секрета хранит то, что нельзя
// восстановить: вид креденшела (чем его предъявлять и где проверять), id токена
// (по нему выход отзывает токен на платформе) и e-mail — под кем вошли.
type Credential struct {
	Kind      CredentialKind `json:"kind"`
	Token     string         `json:"token"`
	TokenID   string         `json:"token_id,omitempty"`
	Email     string         `json:"email,omitempty"`
	ExpiresAt time.Time      `json:"expires_at,omitempty"`
}

// PersonalToken — личный токен пользователя на платформе (метаданные, без
// секрета): им CLI подтверждает, что сохранённый вход ещё жив.
type PersonalToken struct {
	ID        string
	Name      string
	ExpiresAt time.Time
}
