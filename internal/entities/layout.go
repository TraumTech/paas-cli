package entities

import "path/filepath"

// ProtocolFileName — имя файла контракта внутри директории сервиса в раскладке
// потребителя (<destination>/<service-name>/openapi.json).
const ProtocolFileName = "openapi.json"

// ContractSnapshotPath — путь к снимку контракта сервиса в раскладке потребителя.
// Общая раскладка для записи (sync кладёт контракты сюда) и чтения (регистрация
// зависимостей из манифеста берёт снимки отсюда же).
func ContractSnapshotPath(destDir, serviceName string) string {
	return filepath.Join(destDir, serviceName, ProtocolFileName)
}
