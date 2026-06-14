# paas-cli

Универсальный для любого потребителя бинарь, которым получают контракты сервисов
платформы PaaS. Не привязан к стеку потребителя: контракт любого сервиса тянется
одной командой откуда угодно.

## Установка

```sh
go install github.com/TraumTech/paas-cli/cmd/paas-cli@latest
```

Или собрать локально:

```sh
go build -o paas-cli ./cmd/paas-cli
```

## Использование

Получить **актуальный** опубликованный контракт сервиса и записать его к себе.
Имя сервиса CLI узнаёт у платформы и кладёт контракт в
`<destination>/<service-name>/openapi.json`:

```sh
paas-cli protocols fetch <service-id>                      # → protocols/<service-name>/openapi.json
paas-cli --destination vendor/api protocols fetch <service-id>   # → vendor/api/<service-name>/openapi.json
```

Директория для контрактов задаётся глобальным флагом `--destination`/`-d`
(по умолчанию `protocols`). Адрес платформы по умолчанию — прод; переопределяется
переменной окружения `PAAS_API_URL`:

```sh
PAAS_API_URL=http://localhost:8080 paas-cli protocols fetch <service-id>
```

### Гарантии

- **Воспроизводимость.** Повторный прогон после публикации нового контракта
  приносит свежий контракт.
- **Понятные ошибки.** Если контракт не получить (сервис недоступен, не найден или
  контракт ещё не опубликован) — печатается понятная ошибка с ненулевым кодом
  выхода.
- **Не затирает рабочий контракт.** Запись атомарна (временный файл + rename):
  при ошибке/битом ответе ранее полученный контракт остаётся нетронутым.

Кодоген (генерация клиента/типов из контракта) — забота потребителя; CLI только
получает контракт.

## Архитектура

Чистая архитектура (см. `../docs/go-architecture.md`), адаптированная под CLI:

- `internal/entities` — `Protocol` и доменные ошибки;
- `internal/usecases` — use case `FetchProtocol` и интерфейсы зависимостей
  (`ProtocolSource`, `ProtocolStore`);
- `internal/controllers/fetch_command_cli` — подкоманда `protocols fetch`
  (input-адаптер на [urfave/cli v3](https://cli.urfave.org));
- `internal/adapters/protocol_source_http` — HTTP-клиент к API платформы;
- `internal/adapters/protocol_store_file` — атомарная запись протокола в файл;
- `internal/app` — composition root: сборка команды и запуск.

## Разработка

```sh
go generate ./...   # перегенерировать моки (go.uber.org/mock)
go test ./...
```
