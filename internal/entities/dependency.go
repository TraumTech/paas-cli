package entities

import "time"

// Dependency — зарегистрированная в реестре зависимость версии потребителя от
// контракта продьюсера: факт «эта версия собрана на этом контракте». CLI
// регистрирует её из процесса выкатки снимком контракта из репозитория.
type Dependency struct {
	ConsumerVersionID string
	ProducerServiceID string
	RegisteredAt      time.Time
}
