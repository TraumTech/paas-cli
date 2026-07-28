package entities

// AccessRule — правило доступа, которое платформа просит в кластере. Форма
// повторяет правило роли Kubernetes: команда строит из него объект и применяет,
// ничего не разбирая.
type AccessRule struct {
	APIGroups []string
	Resources []string
	Verbs     []string
	// Comment — зачем эти права; показывается владельцу перед применением.
	Comment string
}

// ClusterCredential — то, что команда выдаёт платформе: координаты кластера и
// токен созданной для неё учётной записи. Личный доступ владельца сюда не
// попадает — он служит только для того, чтобы эту учётку завести.
type ClusterCredential struct {
	Endpoint      string
	CACertificate string
	Token         string
}

// ConnectedCluster — подключение глазами платформы после регистрации.
type ConnectedCluster struct {
	ID        string
	Name      string
	Endpoint  string
	Connected bool
}
