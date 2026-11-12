package entities

import "time"

// RegistryLogin — вход в реестр образов платформы, выданный CI сервиса
// (EPIC-31): docker login <Server> -u <Username> -p <Password>.
type RegistryLogin struct {
	Server     string
	Username   string
	Password   string
	Repository string
	ExpiresAt  time.Time
}
