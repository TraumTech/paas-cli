package entities

import (
	"sort"
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
	// Branch — ветка сборки (DEP-17); пусто — публиковали без неё.
	Branch    string
	CreatedAt time.Time
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
	// Variables/Replicas — значения, объявленные манифестом и уже разрешённые
	// под окружение версии (DEP-14/15). Пусто и ноль — версия их не объявляет.
	Variables []FormVariable
	Replicas  int
}

// FormVariable — объявленная переменная окружения; секретов среди них нет.
type FormVariable struct {
	Name  string
	Value string
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

// FormDeclaration — форма, как она объявлена в paas.toml: процессы и значения
// по окружениям (DEP-14/15). Публикуется не она, а разрешённая под конкретное
// окружение VersionForm — версия принадлежит окружению (DEP-08), поэтому
// слияние делает публикующий, а платформа получает готовые значения и не
// хранит правило слияния.
type FormDeclaration struct {
	Processes []ProcessForm
	// Environments — секции [env.*]: ключ DefaultEnvironmentKey несёт общие
	// значения, остальные ключи — окружения платформы.
	Environments map[string]EnvironmentValues
}

// EnvironmentValues — значения одной секции [env.*].
type EnvironmentValues struct {
	Variables map[string]string
	// Replicas 0 — секция числа реплик не задаёт.
	Replicas int
}

// DefaultEnvironmentKey — секция общих значений: [env.default].
const DefaultEnvironmentKey = "default"

// Validate отсекает секции для неизвестных окружений до обращения к платформе:
// опечатка в [env.prd] иначе молча не доехала бы никуда.
func (d *FormDeclaration) Validate() error {
	for name := range d.Environments {
		if name == DefaultEnvironmentKey {
			continue
		}
		if err := ValidateEnvironment(name); err != nil {
			return ErrUnknownFormEnvironment
		}
	}
	return nil
}

// Resolve — форма для публикации в конкретное окружение: общие значения из
// [env.default], поверх — [env.<окружение>]. Переменные сливаются по имени:
// секция окружения перебивает одноимённое общее значение, остальные остаются.
func (d *FormDeclaration) Resolve(environment string) *VersionForm {
	form := &VersionForm{Processes: d.Processes}

	merged := map[string]string{}
	for _, key := range []string{DefaultEnvironmentKey, environment} {
		values, ok := d.Environments[key]
		if !ok {
			continue
		}
		for name, value := range values.Variables {
			merged[name] = value
		}
		if values.Replicas != 0 {
			form.Replicas = values.Replicas
		}
	}

	// Порядок детерминирован: иначе одна и та же ревизия публиковала бы разные
	// формы от запуска к запуску.
	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		form.Variables = append(form.Variables, FormVariable{Name: name, Value: merged[name]})
	}
	return form
}
