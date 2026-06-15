# paas-cli

Универсальный для любого потребителя бинарь, которым получают контракты сервисов
платформы PaaS. Не привязан к стеку потребителя: контракт любого сервиса тянется
одной командой откуда угодно.

## Установка

**macOS — одной командой.** Платформа отдаёт установщик, который сам определяет
архитектуру (Apple Silicon / Intel), скачивает актуальный релиз и ставит бинарь:

```sh
curl -fsSL https://api.paas.traumtech.ru/cli/install.sh | sh
```

Ставится без `sudo` в `~/.local/bin` (каталог переопределяется переменной
`PAAS_CLI_INSTALL_DIR`); если его нет в `PATH`, установщик подскажет, как добавить.
Версию берёт из последнего
[релиза](https://github.com/TraumTech/paas-cli/releases) через бэкенд платформы.

Альтернативы (любая ОС): готовые бинари (linux/macOS/windows, amd64/arm64) — в
[GitHub Releases](https://github.com/TraumTech/paas-cli/releases). Или через Go:

```sh
go install github.com/TraumTech/paas-cli/cmd/paas-cli@latest
```

Или собрать локально:

```sh
go build -o paas-cli ./cmd/paas-cli
```

Версия бинаря: `paas-cli --version`.

## Релизы

Релизы собирает [GoReleaser](https://goreleaser.com) по git-тегу `vX.Y.Z`
(`.github/workflows/release.yml`): кросс-платформенные бинари и архивы
публикуются в GitHub Releases, версия прошивается в бинарь через ldflags.

```sh
git tag v0.1.0 && git push origin v0.1.0   # триггерит сборку релиза
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

### Проверка совместимости контракта-кандидата (для владельца сервиса)

Owner-команда для процесса выкатки: проверить, **ломает** ли контракт-кандидат
зарегистрированных потребителей сервиса — **до** деплоя и **без публикации**.
Платформа сверяет кандидата со снимком каждого потребителя и возвращает разбор;
команда печатает его и завершается ненулевым кодом, если кандидат ломающий.

```sh
paas-cli protocols compatibility <service-id> <candidate-file>   # напр. ./openapi.json
paas-cli protocols compat <service-id> <candidate-file>          # короткий алиас
```

- По каждому потребителю видно, какую его версию затрагивает кандидат, что меняется
  и что из этого ломающее.
- **Нет потребителей** → «кандидат никого не затрагивает», код выхода `0`.
- **Ломающий кандидат** → ненулевой код выхода: пайплайн CI/CD останавливается
  до деплоя. Совместимый → код `0`.
- Несравнимый снимок потребителя ломающим не считается.
- Пустой/неразбираемый файл кандидата, недоступная платформа или ненайденный сервис
  — понятная ошибка (отличимая по выводу от вердикта «ломает»).

Проверка **ничего не публикует и не меняет реестр** — её можно прогонять сколько
угодно раз.

### Публикация версии (для владельца сервиса)

Owner-команда для процесса выкатки: по факту выкатки зафиксировать в реестре новую
версию сервиса, привязанную к развёрнутой ревизии коммита. Заменяет самодельный
`curl`-шаг пайплайна.

```sh
paas-cli versions publish <service-id> <commit-revision>   # напр. "$GITHUB_SHA"
```

- **Идентификатор версии печатается на stdout** отдельной строкой — его без разбора
  подхватывает следующий шаг выкатки (например, привязка протокола к версии):

  ```sh
  version_id=$(paas-cli versions publish "$PAAS_SERVICE_ID" "$GITHUB_SHA")
  ```

  Человекочитаемое подтверждение уходит на stderr, чтобы не мешать автоматике.
- **Идемпотентно**: одна ревизия — одна версия. Повторный прогон с той же ревизией
  («Re-run» пайплайна) возвращает ту же версию с тем же id, а не создаёт дубликат.
- Пустая ревизия, недоступная платформа или ненайденный сервис — понятная ошибка
  и ненулевой код выхода (id на stdout не печатается).

## Архитектура

Чистая архитектура (см. `../docs/go-architecture.md`), адаптированная под CLI:

- `internal/entities` — `Protocol`, `CandidateContract`, `CompatibilityReport`,
  `Version` и доменные ошибки;
- `internal/usecases` — use cases `FetchProtocol` / `CheckCompatibility` /
  `PublishVersion` и интерфейсы зависимостей (`ProtocolSource`, `ProtocolStore`,
  `CandidateReader`, `CompatibilitySource`, `VersionPublisher`);
- `internal/controllers/protocol_fetch_command_cli` — подкоманда `protocols fetch`,
  `internal/controllers/protocol_compatibility_command_cli` — подкоманда
  `protocols compatibility`, `internal/controllers/version_publish_command_cli` —
  подкоманда `versions publish` (input-адаптеры на [urfave/cli v3](https://cli.urfave.org));
- `internal/adapters/protocol_source_http` / `protocol_compatibility_http` /
  `version_publisher_http` — обращение к API платформы через сгенерированный клиент
  `pkg/platformapi`;
- `internal/adapters/protocol_store_file` — атомарная запись протокола в файл,
  `internal/adapters/candidate_reader_file` — чтение файла контракта-кандидата;
- `internal/app` — composition root: сборка команд и запуск;
- `pkg/platformapi` — клиент API платформы, **сгенерированный из контракта**
  ([oapi-codegen](https://github.com/oapi-codegen/oapi-codegen)).

## Догфудинг контракта

`paas-cli` — сам потребитель API бэкенда, поэтому клиент платформы не пишется
руками, а генерируется из контракта, полученного этим же `paas-cli`:

```sh
paas-cli protocols fetch <paas-backend-service-id>   # → protocols/paas-backend/openapi.json
go generate ./...                                    # контракт → pkg/platformapi + моки
```

И контракт (`protocols/paas-backend/openapi.json`), и сгенерированный клиент
(`pkg/platformapi/platformapi.gen.go`) закоммичены — сборка не требует ни сети, ни
повторной генерации.

## Разработка

```sh
go generate ./...   # клиент платформы (oapi-codegen) + моки (go.uber.org/mock)
go test ./...
```
