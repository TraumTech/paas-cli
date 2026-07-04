package entities

import "path/filepath"

// ProtocolFileName — имя файла OpenAPI-контракта внутри директории сервиса в
// раскладке потребителя (<destination>/<service-name>/openapi.json).
const ProtocolFileName = "openapi.json"

// GRPCProtocolFileName — имя файла gRPC-контракта в той же раскладке: контракт
// лежит в родном виде, .proto-исходником (<destination>/<service-name>/contract.proto).
const GRPCProtocolFileName = "contract.proto"

// ProtocolFileNameFor — имя файла контракта в раскладке по его формату.
func ProtocolFileNameFor(format ProtocolFormat) string {
	if format == ProtocolFormatGRPC {
		return GRPCProtocolFileName
	}
	return ProtocolFileName
}

// ContractSnapshotPath — путь к снимку контракта сервиса в раскладке потребителя.
// Общая раскладка для записи (sync кладёт контракты сюда) и чтения (регистрация
// зависимостей из манифеста берёт снимки отсюда же). Регистрация читает только
// OpenAPI-снимки — зависимости с gRPC-снимками появятся вместе с методами
// gRPC-идентичности (roadmap EPIC-04).
func ContractSnapshotPath(destDir, serviceName string) string {
	return filepath.Join(destDir, serviceName, ProtocolFileName)
}
