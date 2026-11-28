package entities

// DefaultLogPushURL — адрес приёма логов платформы.
const DefaultLogPushURL = "https://logs.stapel.sh/loki/api/v1/push"

// LogCollector — сборщик вывода сервисов в кластере приложений (TRA-559):
// читает вывод подов на нодах и шлёт платформе. Чей это кластер, платформа
// узнаёт по ключу, поэтому id кластера сборщику не нужен.
type LogCollector struct {
	PushURL string
	Key     string
}
