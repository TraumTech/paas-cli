package entities

// UserSession — сессия вошедшего пользователя у identity-провайдера: токен для
// предъявления платформе и e-mail владельца для отчёта «под кем вошёл».
type UserSession struct {
	Token string
	Email string
}
