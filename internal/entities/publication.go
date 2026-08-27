package entities

// ProtocolPublication — итог публикации протокола под версией: к какой версии
// привязан контракт и как он совместим с зарегистрированными потребителями.
// Breaking — сводный признак, ломает ли публикация хотя бы одного потребителя;
// он только информирует и публикацию не отменяет (гейт ломающих изменений — в
// отдельной проверке совместимости до деплоя).
// Name — имя (alias), под которым протокол опубликован; пусто — протокол по
// умолчанию (запись прежней формы contract в [service]).
type ProtocolPublication struct {
	Name          string
	VersionNumber int
	Breaking      bool
	Consumers     []ConsumerCompatibility
}

// ProtocolPublishReport — итог публикации всех контрактов манифеста (CLI-23).
// Orphaned — как в ManifestCompatibilityReport: осиротевшие протоколы без
// потребителей, о которых публикация предупреждает, не трогая их.
type ProtocolPublishReport struct {
	Publications []ProtocolPublication
	Orphaned     []string
}
