package entities

import (
	"strings"
	"time"
)

// Version — зафиксированная в реестре версия сервиса. Версия принадлежит
// окружению (DEP-08): одна развёрнутая ревизия — одна версия окружения. CLI
// публикует её из процесса выкатки и отдаёт ID следующему шагу (привязке
// протокола к версии).
type Version struct {
	ID             string
	Environment    string
	Number         int
	CommitRevision string
	CreatedAt      time.Time
}

// Environments — окружения платформы; словарь совпадает с DEP-01.
var Environments = []string{"dev", "stage", "prod"}

// ValidateEnvironment отсекает неизвестное окружение до обращения к платформе.
func ValidateEnvironment(environment string) error {
	for _, env := range Environments {
		if environment == env {
			return nil
		}
	}
	return ErrUnknownEnvironment
}

// VersionForm — форма сервиса из paas.toml (DEP-02): набор процессов, из
// которых платформа сгенерирует манифесты выкатки. Публикуется вместе с
// версией и адресом образа; правила валидации — у платформы, CLI только
// собирает и передаёт.
type VersionForm struct {
	Processes []ProcessForm
}

// ProcessForm — один процесс сервиса. Listen 0 — воркер (ничего не слушает).
// Zone/Prefix — маршрут слушающего процесса (DEP-10): адрес объявляется в
// манифесте и едет с версией, а не назначается в интерфейсе.
type ProcessForm struct {
	Name    string
	Listen  int
	Command []string
	CPU     string
	Memory  string
	Zone    string
	Prefix  string
}

// VersionRequest — намерение зафиксировать версию по развёрнутой ревизии коммита.
type VersionRequest struct {
	CommitRevision string
}

// Validate отсекает пустую ревизию до обращения к платформе: без ревизии версию
// не к чему привязать.
func (r VersionRequest) Validate() error {
	if strings.TrimSpace(r.CommitRevision) == "" {
		return ErrEmptyCommitRevision
	}
	return nil
}
