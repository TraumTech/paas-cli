package entities

// ProtocolPublication — итог публикации протокола под версией: к какой версии
// привязан контракт и как он совместим с зарегистрированными потребителями.
// Breaking — сводный признак, ломает ли публикация хотя бы одного потребителя;
// он только информирует и публикацию не отменяет (гейт ломающих изменений — в
// отдельной проверке совместимости до деплоя).
type ProtocolPublication struct {
	VersionNumber int
	Breaking      bool
	Consumers     []ConsumerCompatibility
}
