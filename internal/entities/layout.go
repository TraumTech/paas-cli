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

// ContractSnapshotPath — путь к снимку контракта сервиса указанного формата в
// раскладке потребителя. Общая раскладка для записи (sync кладёт контракты сюда)
// и чтения (регистрация зависимостей из манифеста берёт снимки отсюда же);
// формат снимка в раскладке различается именем файла (CLI-19/CLI-20).
func ContractSnapshotPath(destDir, serviceName string, format ProtocolFormat) string {
	return filepath.Join(destDir, serviceName, ProtocolFileNameFor(format))
}
