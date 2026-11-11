package entities

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// DatabaseServer — СУБД оператора CloudNativePG в кластере, найденная по имени.
type DatabaseServer struct {
	Namespace string
	Name      string
	// Roles — роли из spec.managed.roles CR СУБД.
	Roles []string
	// Databases — базы из ресурсов Database этой СУБД.
	Databases []string
}

func (s DatabaseServer) HasRole(name string) bool {
	return slices.Contains(s.Roles, name)
}

// DatabaseCredentials — вход личной роли, печатается для клиента БД.
type DatabaseCredentials struct {
	User     string
	Password string
}

var personalRolePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// reservedRoles — роли, которые заводит сам CNPG: личная роль их не подменяет.
var reservedRoles = []string{"postgres", "app", "streaming_replica"}

// ValidatePersonalRole проверяет имя личной роли владельца в СУБД.
func ValidatePersonalRole(name string) error {
	if !personalRolePattern.MatchString(name) ||
		slices.Contains(reservedRoles, name) ||
		strings.HasPrefix(name, "pg_") || strings.HasPrefix(name, "cnpg_") {
		return ErrInvalidPersonalRole
	}
	return nil
}

// AmbiguousDatabaseServerError — СУБД с таким именем есть в нескольких
// namespace: в общем кластере одно имя может быть у разных организаций.
type AmbiguousDatabaseServerError struct {
	Name       string
	Namespaces []string
}

func (e *AmbiguousDatabaseServerError) Error() string {
	return fmt.Sprintf("СУБД %s есть в нескольких namespace (%s) — укажите нужный флагом --namespace",
		e.Name, strings.Join(e.Namespaces, ", "))
}

// DatabaseRoleRejectedError — оператор не смог завести роль.
type DatabaseRoleRejectedError struct {
	Role   string
	Reason string
}

func (e *DatabaseRoleRejectedError) Error() string {
	return fmt.Sprintf("оператор СУБД не завёл роль %s: %s", e.Role, e.Reason)
}
