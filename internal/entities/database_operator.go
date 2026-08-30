package entities

// DatabaseOperator — оператор Kubernetes, которым платформа заводит СУБД
// данного типа в подключённом кластере (DB-05). Приходит от платформы целиком:
// что именно попадёт в кластер владельца, определяет она, а не CLI.
type DatabaseOperator struct {
	Engine  string
	Name    string
	Version string
	// Manifest — полный релизный манифест оператора (многодокументный YAML).
	Manifest string
	// Namespace и Deployment — по чему видно, что оператор поднялся.
	Namespace  string
	Deployment string
	// Rules — что платформе нужно поверх обычных прав в кластере, чтобы
	// заводить СУБД этого типа.
	Rules []AccessRule
}

// ManifestObject — один объект манифеста; показывается владельцу до применения.
type ManifestObject struct {
	Kind      string
	Namespace string
	Name      string
}

// ObjectChange — что стало с объектом после применения.
type ObjectChange struct {
	ManifestObject
	Change Change
}

type Change string

const (
	ChangeCreated   Change = "created"
	ChangeUpdated   Change = "updated"
	ChangeUnchanged Change = "unchanged"
)

// OperatorInstallReport — итог установки: по объектам, чтобы повтор честно
// говорил «менять нечего», а обновление — что именно обновилось.
type OperatorInstallReport struct {
	Changes []ObjectChange
}

func (r OperatorInstallReport) Count(change Change) int {
	n := 0
	for _, c := range r.Changes {
		if c.Change == change {
			n++
		}
	}
	return n
}
