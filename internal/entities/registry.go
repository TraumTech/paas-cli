package entities

import (
	"fmt"
	"strings"
)

// RegistryProtocol — текущий протокол сервиса в реестре платформы: имя (alias)
// и формат последней публикации (PRT-22).
type RegistryProtocol struct {
	Name   string
	Format string
}

// RegisteredConsumer — зарегистрированный потребитель контракта сервиса.
type RegisteredConsumer struct {
	ServiceName   string
	VersionNumber int
}

// OrphanedProtocols возвращает протоколы реестра, исчезнувшие из перечня
// манифеста (CLI-23): опубликованы, но не объявлены ни одной записью. Запись
// без имени — протокол по умолчанию. Исчезновение — в том числе след
// переименования имени в манифесте: для реестра оно неотличимо от «старый
// исчез, новый появился».
func OrphanedProtocols(published []RegistryProtocol, declared []ManifestProtocol) []RegistryProtocol {
	names := make(map[string]struct{}, len(declared))
	for _, p := range declared {
		name := p.Name
		if name == "" {
			name = DefaultProtocolName
		}
		names[name] = struct{}{}
	}
	var orphans []RegistryProtocol
	for _, p := range published {
		if _, ok := names[p.Name]; !ok {
			orphans = append(orphans, p)
		}
	}
	return orphans
}

// OrphanedProtocolError — протокол исчез из манифеста, а от него зависят
// потребители: для них это ломающее изменение, публикация удерживается (PRT-09).
type OrphanedProtocolError struct {
	Name      string
	Consumers []RegisteredConsumer
}

func (e *OrphanedProtocolError) Error() string {
	names := make([]string, 0, len(e.Consumers))
	for _, c := range e.Consumers {
		names = append(names, fmt.Sprintf("%s v%d", c.ServiceName, c.VersionNumber))
	}
	return fmt.Sprintf("протокол %q опубликован в реестре, но исчез из манифеста, а от него зависят потребители: %s — верните запись в перечень либо снимите зависимость потребителей",
		e.Name, strings.Join(names, ", "))
}
