package entities

import (
	"strings"
	"time"
)

// Version — зафиксированная в реестре версия сервиса: одна развёрнутая ревизия
// коммита соответствует ровно одной версии. CLI публикует её из процесса выкатки
// и отдаёт ID следующему шагу (привязке протокола к версии).
type Version struct {
	ID             string
	Number         int
	CommitRevision string
	CreatedAt      time.Time
}

// VersionForm — форма сервиса из paas.toml (DEP-02): набор процессов, из
// которых платформа сгенерирует манифесты выкатки. Публикуется вместе с
// версией и адресом образа; правила валидации — у платформы, CLI только
// собирает и передаёт.
type VersionForm struct {
	Processes []ProcessForm
}

// ProcessForm — один процесс сервиса. Listen 0 — воркер (ничего не слушает).
type ProcessForm struct {
	Name    string
	Listen  int
	Command []string
	CPU     string
	Memory  string
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
