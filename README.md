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

#### Частичный контракт

Если из контракта нужны лишь отдельные методы, перечисли их operationId флагом
`--method`/`-m` (повторяемый или через запятую) — CLI обрежет контракт до этих
операций. Каждая оставленная операция сохраняется целиком, а определения, на
которые срез больше не ссылается, отбрасываются: результат самодостаточен и меньше:

```sh
paas-cli protocols fetch <service-id> -m create-order -m list-orders
paas-cli protocols fetch <service-id> --method create-order,list-orders
```

Если указан несуществующий метод, команда сообщает, какой именно не найден, и
ничего не записывает — рабочий контракт не затирается неполным срезом. Набор
методов задаётся явно, поэтому прогон воспроизводим.

Директория для контрактов задаётся глобальным флагом `--destination`/`-d`
(по умолчанию `protocols`). Адрес платформы по умолчанию — прод; переопределяется
переменной окружения `PAAS_API_URL`:

```sh
PAAS_API_URL=http://localhost:8080 paas-cli protocols fetch <service-id>
```

#### Манифест зависимостей

Чтобы не перечислять сервисы в команде при каждом обновлении, состав зависимостей
объявляется в манифесте `protocols.toml` в корне репозитория потребителя, а
`protocols sync` тянет все объявленные контракты разом. Манифест перечисляет сервисы
**по имени** (не по версии): воспроизводимость даёт git — полученные снимки
коммитятся, — а не пин на эфемерную версию продьюсера.

```toml
# protocols.toml
# destination опционально (по умолчанию "protocols")
# destination = "protocols"

[[dependencies]]
name = "paas-backend"

[[dependencies]]
name = "billing"
methods = ["create-invoice", "list-invoices"]   # необязательный частичный контракт
```

```sh
paas-cli protocols sync                       # читает ./protocols.toml
paas-cli protocols sync --manifest deps.toml  # другой путь к манифесту (-f)
```

- Раскладка та же, что у `fetch`: по контракту на сервис в `<destination>/<name>/openapi.json`.
  Директорию задаёт `destination` в манифесте; явный `--destination`/`-d` её переопределяет.
- На каждую зависимость можно сузить контракт до методов полем `methods` (как `-m` у `fetch`).
- Имя сервиса CLI резолвит в его id у платформы. Если объявленный сервис не найден,
  контракт не получить, либо манифест отсутствует/пуст/неразбираем — команда печатает
  понятную ошибку (с именем упавшей зависимости) и завершается ненулевым кодом,
  останавливая пайплайн; уже полученные рабочие контракты не затираются.

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

### Публикация протокола (для владельца сервиса)

Owner-команда для процесса выкатки: опубликовать протокол сервиса под версией —
контракт берётся из **локального файла репозитория**, а не выкачивается из
запущенного сервиса. Платформа привязывает протокол к версии и возвращает разбор
совместимости с потребителями, который команда печатает.

```sh
paas-cli protocols publish <service-id> <version-id> <contract-file>   # напр. ./openapi.json
```

- Версию (`<version-id>`) даёт предыдущий шаг выкатки — публикация версии по
  развёрнутой ревизии (см. roadmap эпика CLI).
- После публикации печатается сводка совместимости: по каждому потребителю — какую
  его версию затрагивает контракт, что меняется и что из этого ломающее.
- **Нет потребителей** → «публикация никого не затрагивает».
- Сводка **только информирует**: ломающее изменение не делает команду неуспешной
  (код выхода `0`) — гейт ломающих изменений живёт в проверке совместимости
  (`protocols compatibility`) до деплоя.
- Пустой/неразбираемый файл контракта, недоступная платформа, ненайденный сервис или
  версия — понятная ошибка с ненулевым кодом выхода.

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

### Регистрация зависимости версии от контракта (для потребителя)

Команда потребителя для процесса выкатки: объявить в реестре, что зафиксированная
версия собрана на контракте другого сервиса, приложив снимок этого контракта из
**локального файла репозитория**. Заменяет самодельный `jq`+`curl`-шаг пайплайна.

```sh
paas-cli dependencies register <service-id> <version-id> <producer-service-id> <contract-file>
```

- `<service-id>`/`<version-id>` — собственный сервис-потребитель и его версия (id
  версии даёт `versions publish`); `<producer-service-id>` — сервис, на контракте
  которого собрана версия; `<contract-file>` — снимок контракта продьюсера (напр.
  `protocols/<producer>/openapi.json`, полученный `protocols fetch`).
- **Идемпотентно**: повторная регистрация той же версии на того же продьюсера
  обновляет снимок, а не плодит дубль («Re-run» пайплайна безопасен).
- Пустой/неразбираемый файл контракта, недоступная платформа, ненайденная версия
  или продьюсер — понятная ошибка с ненулевым кодом выхода.

## Архитектура

Чистая архитектура (см. `../docs/go-architecture.md`), адаптированная под CLI:

- `internal/entities` — `Protocol`, `CandidateContract`, `CompatibilityReport`,
  `Version` и доменные ошибки;
- `internal/usecases` — use cases `FetchProtocol` / `CheckCompatibility` /
  `PublishVersion` и интерфейсы зависимостей (`ProtocolSource`, `ProtocolStore`,
  `CandidateReader`, `CompatibilitySource`, `VersionPublisher`);
- `internal/entities` — `Protocol`, `CandidateContract`, `CompatibilityReport`,
  `ProtocolPublication` и доменные ошибки;
- `internal/usecases` — use cases `FetchProtocol` / `CheckCompatibility` /
  `PublishProtocol` и интерфейсы зависимостей (`ProtocolSource`, `ProtocolStore`,
  `CandidateReader`, `CompatibilitySource`, `ProtocolPublisher`);
- `internal/controllers/protocol_fetch_command_cli` — подкоманда `protocols fetch`,
  `internal/controllers/protocol_compatibility_command_cli` — подкоманда
  `protocols compatibility`, `internal/controllers/protocol_publish_command_cli` —
  подкоманда `protocols publish` (input-адаптеры на [urfave/cli v3](https://cli.urfave.org));
- `internal/adapters/protocol_source_http` / `protocol_compatibility_http` /
  `protocol_publish_http` — обращение к API платформы через сгенерированный клиент
  `pkg/platformapi`;
  `protocols compatibility`, `internal/controllers/version_publish_command_cli` —
  подкоманда `versions publish` (input-адаптеры на [urfave/cli v3](https://cli.urfave.org));
- `internal/adapters/protocol_source_http` / `protocol_compatibility_http` /
  `version_publisher_http` — обращение к API платформы через сгенерированный клиент
  `pkg/platformapi`;
- `internal/adapters/protocol_store_file` — атомарная запись протокола в файл,
  `internal/adapters/candidate_reader_file` — чтение файла контракта с диска;
- манифест зависимостей: `entities.Manifest` + use case `SyncProtocols`
  (интерфейсы `ManifestReader`, `ServiceResolver`); подкоманда
  `internal/controllers/protocol_sync_command_cli` (`protocols sync`);
  адаптеры `internal/adapters/manifest_reader_file` (разбор `protocols.toml`) и
  `internal/adapters/service_resolver_http` (резолв имени сервиса в id);
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
