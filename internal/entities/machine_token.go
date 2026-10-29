package entities

import "strings"

// MachineToken — токен доступа из PAAS_API_TOKEN (IAP-04): токен сервиса или
// личный токен платформы, выпущенный как клиент входа платформы. Значение
// одно — `paas_pat_<id>.<секрет>` (личный — `paas_uat_…`): часть до точки —
// идентификатор клиента, после — его секрет. CLI меняет пару на короткий JWT
// (client_credentials) и предъявляет платформе его, а не сам секрет.
type MachineToken struct {
	ClientID string
	Secret   string
}

// ParseMachineToken разбирает значение PAAS_API_TOKEN. false — это секрет
// прежнего вида (без точки): его платформа ещё принимает как есть, но JWT за
// него не выдаёт.
func ParseMachineToken(value string) (MachineToken, bool) {
	clientID, secret, ok := strings.Cut(value, ".")
	if !ok || clientID == "" || secret == "" {
		return MachineToken{}, false
	}
	return MachineToken{ClientID: clientID, Secret: secret}, true
}
