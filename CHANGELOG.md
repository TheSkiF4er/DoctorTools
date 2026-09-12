## 3.0.1 — release hygiene patch

- DoctorTools обновлён до 3.0.1.
- CraftDoctor обновлён до 3.0.1.
- StackDoctor, DepDoctor, ConfigDoctor, DeployDoctor, APIDoctor, BotDoctor и LogDoctor обновлены до 1.0.1.
- Исправлена релизная упаковка: `bin/*` и `scripts/*.sh` получают execute-bit при сборке.
- `Makefile` теперь явно выставляет права запуска для бинарников после `go build`.
- `scripts/release.sh` нормализует timestamps релизных файлов и сохраняет Unix-права в Linux/macOS пакетах.
- `schemas/rules-catalog.schema.json` обновлена под DoctorTools 3.x и product/profile-aware правила.
- Исправлена история релизов: блок усиления CraftDoctor возвращён к версии 2.9.0.
- GitFlic Wiki очищена от устаревших формулировок DoctorTools 2.8/2.9 и приведена к 3.0.1.

## 3.0.0 — стабильный platform-релиз DoctorTools

- DoctorTools обновлён до 3.0.0.
- CraftDoctor обновлён до 3.0.0.
- StackDoctor, DepDoctor, ConfigDoctor, DeployDoctor, APIDoctor, BotDoctor и LogDoctor обновлены до 1.0.0.
- Добавлена проверка реестра продуктов: `doctortools doctor health` и `doctortools doctor health --json`.
- ConfigDoctor вынесен в самостоятельный пакет `internal/products/configdoctor` и больше не запускается через preview-слой `simpledoctor`.
- DeployDoctor вынесен в самостоятельный пакет `internal/products/deploydoctor` и больше не запускается через preview-слой `simpledoctor`.
- Добавлены стабильные helper-функции DoctorCore для JSON-вывода и рекомендуемых exit codes.
- Обновлены версии, smoke/release-скрипты, README, GitFlic Wiki и схемы.

# Журнал изменений

## 2.9.0 — product-релиз усиления CraftDoctor

- DoctorTools обновлён до 2.9.0.
- CraftDoctor обновлён до 2.9.0.
- StackDoctor обновлён до 0.3.3.
- DepDoctor обновлён до 0.3.4.
- ConfigDoctor обновлён до 0.3.6.
- DeployDoctor обновлён до 0.3.5.
- APIDoctor обновлён до 0.3.2.
- BotDoctor обновлён до 0.3.1.
- LogDoctor обновлён до 0.3.7.
- Добавлен пакет `internal/analyzers/craftadvanced`.
- В полный JSON-отчёт CraftDoctor добавлен блок `craft_advanced`.
- Добавлена команда `craftdoctor java flags` для ревизии JVM-флагов, heap, GC и устаревших параметров запуска.
- Добавлена команда `craftdoctor world audit` для проверки миров, `level.dat`, `session.lock`, `uid.dat`, region/entities/playerdata/datapacks.
- Добавлена команда `craftdoctor production gate` для Minecraft-specific production-gate: EULA, heap, proxy/offline-mode, startup и backup.
- Добавлен alias `craftdoctor proxy audit` для proxy/network-аудита.
- Добавлены правила `rules/craftdoctor/advanced.json`.
- Добавлены unit-тесты расширенного CraftDoctor-аудита.
- Обновлены README, GitFlic Wiki, smoke/release-скрипты и бинарники.


## 2.8.0 — product-релиз BotDoctor

- DoctorTools обновлён до 2.8.0.
- CraftDoctor обновлён до 2.8.0.
- BotDoctor обновлён до 1.0.0 и переведён в статус product.
- Добавлен полноценный пакет `internal/products/botdoctor`.
- `botdoctor` больше не использует общий preview-каркас `simpledoctor`.
- `doctortools bot ...` теперь вызывает полноценный BotDoctor.
- Добавлены команды `scan`, `telegram check`, `discord check`, `vk check`, `env audit`, `webhook check`, `bothost check`, `security audit`.
- Добавлен offline-аудит Telegram/VK/Discord bot-проектов: токены, `.env.example`, webhook/polling, logging, rate limit, graceful shutdown, systemd, Docker и BotHost-ready состояние.
- Добавлены JSON-отчёты BotDoctor и unit-тесты.
- Добавлены правила `rules/botdoctor/bots.json`.
- Обновлены README, GitFlic Wiki, release/smoke-скрипты и бинарники.

# CHANGELOG

## 2.7.0 — APIDoctor product release

### Добавлено

- Добавлен полноценный пакет `internal/products/apidoctor`.
- APIDoctor переведён из product-preview в product-статус.
- Добавлены команды `apidoctor scan`, `apidoctor openapi validate`, `apidoctor routes check`, `apidoctor security audit`, `apidoctor health check`, `apidoctor probe`.
- Добавлен offline-first аудит backend/API проектов: OpenAPI/Swagger, маршруты, auth middleware, CORS, rate limit, request validation, error format, pagination и health endpoints.
- Добавлен базовый HTTP probe для URL и health endpoints с настраиваемым timeout.
- Добавлены JSON-отчёты APIDoctor с API-файлами, endpoints, findings, summary и artifacts.
- Добавлены unit-тесты APIDoctor.
- Добавлены правила `rules/apidoctor/api.json`.

### Изменено

- DoctorTools обновлён до `2.7.0`.
- CraftDoctor обновлён до `2.7.0`.
- APIDoctor обновлён до `1.0.0` и получил статус `product`.
- StackDoctor обновлён до `1.0.0`.
- DepDoctor обновлён до `1.0.0`.
- ConfigDoctor обновлён до `1.0.0`.
- DeployDoctor обновлён до `1.0.0`.
- LogDoctor обновлён до `1.0.0`.
- BotDoctor обновлён до `1.0.0`.
- `doctortools api ...` теперь использует полноценный APIDoctor, а не общий simpledoctor-каркас.
- README, GitFlic Wiki, smoke/release-скрипты обновлены под релиз 2.7.0.

### Проверено

- `go test ./...`
- `go vet ./...`
- `make build-all`
- `./scripts/smoke.sh`

## 2.6.0 — StackDoctor product release

### Добавлено

- Добавлен полноценный пакет `internal/products/stackdoctor`.
- StackDoctor переведён из product-preview в product-статус.
- Добавлены команды `stackdoctor scan`, `stackdoctor structure check`, `stackdoctor frontend scan`, `stackdoctor backend scan`, `stackdoctor env audit`, `stackdoctor production check`.
- Добавлен offline-аудит full-stack проектов: структура, frontend/backend компоненты, env, lock-файлы, OpenAPI, Docker, release-документы, тестовые сигналы и production-readiness.
- Добавлен анализ директорий проекта и ZIP-архивов без необходимости распаковки.
- Добавлены JSON-отчёты StackDoctor с компонентами, findings, summary и artifacts.
- Добавлены unit-тесты StackDoctor.
- Добавлены правила `rules/stackdoctor/fullstack.json`.

### Изменено

- DoctorTools обновлён до `2.6.0`.
- CraftDoctor обновлён до `2.6.0`.
- StackDoctor обновлён до `1.0.0` и получил статус `product`.
- DepDoctor обновлён до `1.0.0`.
- ConfigDoctor обновлён до `1.0.0`.
- DeployDoctor обновлён до `1.0.0`.
- LogDoctor обновлён до `1.0.0`.
- APIDoctor и BotDoctor обновлены до `1.0.0`.
- `doctortools stack ...` теперь использует полноценный StackDoctor, а не общий simpledoctor-каркас.
- README, GitFlic Wiki, smoke/release-скрипты обновлены под релиз 2.6.0.

### Проверено

- `go test ./...`
- `go vet ./...`
- `make build-all`
- `./scripts/smoke.sh`

## 2.5.0 — DepDoctor product release

### Добавлено

- Добавлен полноценный пакет `internal/products/depdoctor`.
- DepDoctor переведён из product-preview в product-статус.
- Добавлены команды `depdoctor audit`, `depdoctor lock check`, `depdoctor scripts audit`, `depdoctor licenses`, `depdoctor outdated`, `depdoctor supply-chain scan`.
- Добавлен offline-аудит Node.js, PHP Composer, Go, Python requirements/pyproject, Rust Cargo, Java Maven/Gradle.
- Добавлена проверка lock-файлов, wildcard/latest/moving target версий, pre-release/dev версий, Git/HTTP-зависимостей и широких диапазонов версий.
- Добавлен аудит install/build scripts на `postinstall`, `preinstall`, `curl`, `wget`, pipe-to-shell, `powershell`, `base64 -d` и `chmod +x` паттерны.
- Добавлены JSON-отчёты DepDoctor и unit-тесты.

### Изменено

- DoctorTools обновлён до `2.5.0`.
- CraftDoctor обновлён до `2.5.0`.
- DepDoctor обновлён до `1.0.0` и получил статус `product`.
- ConfigDoctor обновлён до `1.0.0`.
- DeployDoctor обновлён до `1.0.0`.
- LogDoctor обновлён до `1.0.0`.
- StackDoctor, APIDoctor и BotDoctor обновлены до `1.0.0`.
- `doctortools dep ...` теперь использует полноценный DepDoctor, а не общий simpledoctor-каркас.

### Проверено

- `go test ./...`
- `go vet ./...`
- `make build-all`
- `./scripts/smoke.sh`


## 2.4.0 — DeployDoctor product release

### Добавлено

- Добавлен полноценный пакет `internal/products/deploydoctor`.
- DeployDoctor переведён из product-preview в product-статус.
- Добавлены команды `deploydoctor check`, `deploydoctor release check`, `deploydoctor ci check`, `deploydoctor env check`, `deploydoctor docker check`, `deploydoctor archive check`.
- Добавлены pre-deploy проверки README, CHANGELOG, LICENSE, SECURITY, RELEASE_CHECKLIST, CI/Makefile, lock-файлов и Docker readiness.
- Добавлена проверка `.env` / `.env.example` перед релизом.
- Добавлена проверка ZIP-архивов на `.env`, `.git`, `node_modules`, `vendor`, временные файлы и отсутствие README/.env.example.
- Добавлены JSON-отчёты DeployDoctor и unit-тесты.

### Изменено

- DoctorTools обновлён до `2.4.0`.
- CraftDoctor обновлён до `2.4.0`.
- DeployDoctor обновлён до `1.0.0` и получил статус `product`.
- ConfigDoctor обновлён до `1.0.0`.
- LogDoctor обновлён до `1.0.0`.
- StackDoctor, DepDoctor, APIDoctor и BotDoctor обновлены до `1.0.0`.
- `doctortools deploy ...` теперь использует полноценный DeployDoctor, а не общий simpledoctor-каркас.
- README, GitFlic Wiki, smoke/release-скрипты обновлены под релиз 2.4.0.

### Проверено

- `go test ./...`
- `go vet ./...`
- `make build-all`
- `./scripts/smoke.sh`

## 2.3.0 — ConfigDoctor product release

### Добавлено

- Добавлен полноценный пакет `internal/products/configdoctor`.
- ConfigDoctor переведён из product-preview в product-статус.
- Добавлены команды `configdoctor nginx audit`, `configdoctor docker audit`, `configdoctor systemd audit`, `configdoctor env audit`, `configdoctor permissions check`.
- Добавлен рекурсивный поиск Nginx, Docker, Compose, systemd и env-конфигураций.
- Добавлен аудит Nginx reverse proxy headers, TLS-протоколов, HTTP→HTTPS redirect, `client_max_body_size`, access/error logs и рискованных `alias`-конфигураций.
- Добавлен аудит Dockerfile и compose-файлов: latest/unpinned images, отсутствие HEALTHCHECK, root user, отсутствие `.dockerignore`, копирование `.env`, privileged mode и host network.
- Добавлен аудит systemd unit-файлов: ExecStart, Restart, root user и базовые hardening-параметры.
- Добавлен аудит `.env` / `.env.example` на реальные секреты, отсутствие `.env.example` и небезопасные шаблоны.
- Добавлен аудит world-writable прав для файлов и каталогов.
- Добавлены unit-тесты ConfigDoctor.

### Изменено

- DoctorTools обновлён до `2.3.0`.
- CraftDoctor обновлён до `2.3.0`.
- ConfigDoctor обновлён до `1.0.0` и получил статус `product`.
- LogDoctor обновлён до `1.0.0`.
- StackDoctor, DepDoctor, DeployDoctor, APIDoctor и BotDoctor обновлены до `1.0.0`.
- `doctortools config ...` теперь использует полноценный ConfigDoctor, а не общий simpledoctor-каркас.
- README, GitFlic Wiki, smoke/release-скрипты обновлены под релиз 2.3.0.

### Проверено

- `go test ./...`
- `go vet ./...`
- `make build-all`
- `./scripts/smoke.sh`

## 2.2.0 — LogDoctor product release

### Добавлено

- Добавлен полноценный пакет `internal/products/logdoctor`.
- LogDoctor переведён из product-preview в product-статус.
- Добавлен рекурсивный поиск логов `.log`, `.txt`, `.out`, `.err`.
- Добавлен анализ паттернов `fatal`, `critical`, `panic`, `exception`, `stack trace`, `error`, `warning`, `OOM`, `permission denied`, database/network errors.
- Добавлены команды `logdoctor analyze`, `logdoctor summarize`, `logdoctor root-cause`, `logdoctor grep`.
- Добавлены JSON-отчёты LogDoctor с детализацией по файлам, line samples, группами паттернов и summary.
- `doctortools log ...` теперь использует полноценный LogDoctor, а не общий simpledoctor-каркас.
- Добавлены unit-тесты LogDoctor.

### Изменено

- DoctorTools обновлён до `2.2.0`.
- CraftDoctor обновлён до `2.2.0`.
- LogDoctor обновлён до `1.0.0` и получил статус `product`.
- StackDoctor, DepDoctor, ConfigDoctor, DeployDoctor, APIDoctor и BotDoctor обновлены до `1.0.0`.
- Smoke-сценарий дополнен проверкой LogDoctor 1.0.0.
- README и GitFlic Wiki обновлены под релиз 2.2.0.

### Проверено

- `go test ./...`
- `go vet ./...`
- `make build-all`
- `./scripts/smoke.sh`

## 2.1.0 — DoctorCore product release

### Добавлено

- Добавлен пакет `internal/core` с общим DoctorCore Engine.
- Добавлены единые модели `Product`, `Context`, `Analyzer`, `Finding`, `Report`.
- Добавлен общий механизм выполнения профильных анализаторов.
- Добавлен общий writer JSON/text-отчётов.
- Реестр продуктов перенесён в `internal/version`.
- `doctortools version --json`, `doctortools products --json` и `doctortools doctor list --json` используют единый реестр.
- StackDoctor, DepDoctor, ConfigDoctor, DeployDoctor, APIDoctor, BotDoctor и LogDoctor обновлены до product-preview `1.0.0`.
- Для product-preview инструментов добавлены реальные профильные проверки: structure/env, lock-файлы, Docker/Nginx, release checklist, OpenAPI, bot entrypoint, log error/fatal/exception patterns.

### Изменено

- DoctorTools обновлён до `2.1.0`.
- CraftDoctor обновлён до `2.1.0`.
- JSON Schema отчёта переименована в `DoctorTools Report`.
- CLI entrypoints всех product-preview инструментов берут версии из единого реестра.
- Release/smoke scripts переведены на `2.1.0`.

### Проверено

- `go test ./...`
- `go vet ./...`
- `make build-all`
- `./scripts/smoke.sh`

# Журнал изменений

## 2.0.1 — стабилизация DoctorTools Foundation

### Добавлено

- Единый реестр продуктов DoctorTools в `internal/version`.
- Команда `doctortools products`.
- Команда `doctortools doctor list`.
- JSON-вывод реестра продуктов через `--json`.
- Полная сборка всех девяти Linux-бинарников в `bin/`.

### Изменено

- Версия платформы DoctorTools обновлена до `2.0.1`.
- CraftDoctor обновлён до `2.0.1`.
- Foundation-инструменты StackDoctor, DepDoctor, ConfigDoctor, DeployDoctor, APIDoctor, BotDoctor и LogDoctor обновлены до `0.1.1`.
- JSON Schema отчётов обновлена под DoctorTools 2.x и совместимость с CraftDoctor 2.x.
- JSON Schema реестра продуктов расширена полями `key`, `aliases`, `status` и `commands`.
- Релизные скрипты переведены на `2.0.1`.
- `Makefile` автоматически выставляет права запуска для `scripts/*.sh`.

### Исправлено

- Прямой запуск `./scripts/smoke.sh`, `./scripts/release.sh` и других shell-скриптов больше не падает из-за отсутствия execute-бита.
- В документации и схемах устранены устаревшие упоминания `CraftDoctor Report` как единственной схемы платформы.
- `doctortools version` теперь показывает актуальные версии всех продуктов.

### Проверено

- `go test ./...`.
- `go vet ./...`.
- `make build-all`.
- `make smoke`.

## 2.0.0 — DoctorTools Monorepo

- CraftDoctor перенесён в монорепозиторий DoctorTools.
- Добавлен единый бинарник `doctortools`.
- Добавлены CLI-каркасы StackDoctor, DepDoctor, ConfigDoctor, DeployDoctor, APIDoctor, BotDoctor и LogDoctor.
- Сохранён самостоятельный бинарник `craftdoctor` и основные команды CraftDoctor 1.9.0.
- Добавлена общая GitFlic Wiki для DoctorTools.
- Добавлена сборка всех бинарников через `make build-all`.

# Журнал изменений

## 1.9.0

Минорный продуктовый релиз с **HTML Report 3.0**.

### Добавлено

- Расширенная dashboard-сводка HTML-отчёта.
- Score breakdown по штрафам CRITICAL / DANGER / WARN.
- Risk matrix по категориям и уровням серьёзности.
- Поиск по находкам в HTML-отчёте.
- Кнопка печати/сохранения отчёта в PDF средствами браузера.
- Экспортный блок с командами генерации HTML/Markdown/JSON/CI-отчёта.
- Группировка находок по категориям.
- Печатный режим для передачи отчёта клиенту, хостеру или разработчику плагина.
- Новая GitFlic Wiki-страница `html-report.md`.

### Изменено

- Версия инструмента обновлена до `1.9.0`.
- Версия каталога правил обновлена до `1.9.0`.
- HTML-отчёт обновлён с формата 2.0 до 3.0.
- Smoke-сценарий проверяет маркеры HTML Report 3.0.
- Sample reports пересобраны под формат 1.9.0.

### Проверено

- `go test ./...`.
- `go vet ./...`.
- `go build ./cmd/craftdoctor`.
- `scripts/smoke.sh`.
- `scripts/release.sh`.


## 1.8.0

### Добавлено

- GitFlic CI pipeline в `.gitflic-ci.yml`.
- Цель `make lint`: `gofmt`, `go vet`, проверка русскоязычности пользовательских текстов.
- Цель `make check-release`.
- Release manifest `craftdoctor-<version>-manifest.json`.
- Новая Wiki-страница `release-automation.md`.

### Изменено

- Версия инструмента обновлена до `1.8.0`.
- Версия каталога правил обновлена до `1.8.0`.
- `scripts/release.sh` теперь определяет версию из `internal/version/version.go`.
- `scripts/release.sh` запускает `go test`, `go vet`, проверку русскоязычности, проверку каталога правил и JSON Schema перед упаковкой.
- Release-архивы теперь дополнительно проверяются отдельным скриптом.
- Smoke-сценарий проверяет release manifest, SHA256 и release-check pipeline.
- GitFlic Wiki `ci-cd.md` обновлена под GitFlic CI и release pipeline.
- Sample reports пересобраны под формат 1.8.0.

### Проверено

- `make lint`.
- `go test ./...`.
- `go vet ./...`.
- `go build ./cmd/craftdoctor`.
- `scripts/smoke.sh`.
- `scripts/release.sh`.

## 1.7.0

### Добавлено

- Production Doctor 2.0 как расширенный слой проверки production-ready состояния.
- Поддержка `--systemd-dir`, `--logrotate-dir` и `--backup-max-age` для `production check`, `scan` и `ci`.
- Анализ Docker/docker-compose: Dockerfile, compose-файлы, признаки volumes и restart policy.
- Извлечение команд запуска из `start.sh`/`run.sh` и `ExecStart` systemd service-файлов.
- Проверки внешних директорий systemd и logrotate.
- Проверки restore checklist: `RESTORE.md`, `restore.md`, backup/restore и Wiki-страницы восстановления.
- Анализ release layout: `current`, `previous`, `releases`.
- Проверка размера backup-кандидатов и предупреждение о пустых/непроверяемых бэкапах.
- Новые JSON-поля `production.options`, `production.docker`, `production.startup_commands`, `production.restore_checklist`, `production.release_layout`.
- Новые правила `production.docker.*`, `production.restore.*`, `production.release_layout.*`, `production.startup_commands.*`.
- Новая Wiki-страница `production-doctor.md`.

### Изменено

- Версия инструмента обновлена до `1.7.0`.
- Версия каталога правил обновлена до `1.7.0`.
- HTML/Markdown-отчёты Production Doctor расширены до формата 2.0.
- Smoke-сценарий проверяет Production Doctor 2.0 и новые production-флаги.
- Sample reports пересобраны под формат 1.7.0.

### Проверено

- `go test ./...`.
- `go vet ./...`.
- `go build ./cmd/craftdoctor`.
- `scripts/smoke.sh`.
- `scripts/release.sh`.
- `craftdoctor production check --backup-max-age 24h`.
- `craftdoctor production check --format json`.

## 1.6.0

### Добавлено

- Security Doctor 2.0 как расширенный слой аудита безопасности.
- Security score 0–100 в JSON/HTML/Markdown/текстовом выводе.
- `.craftdoctor-secrets-ignore` для осознанного подавления false-positive секретов.
- Расширенный secret scanner: Discord webhook, Telegram bot token, Bearer token, API key, token/secret, password и private key.
- Проверки прав `forwarding.secret`: world-readable и group-readable.
- Проверки чувствительных файлов: `server.properties`, `ops.json`, whitelist/ban-list, `.env`, database/Discord-конфиги.
- Сигналы debug/admin/security/rollback-плагинов.
- Новые JSON-поля `security.score`, `security.secrets_ignore_file`, `security.secret_files_scanned`, `security.sensitive_files`, `security.plugin_signals`.
- Новые правила `security.forwarding_secret.*`, `security.sensitive_file.*`, `security.plugins.*`, `security.secrets.*`.
- Новая Wiki-страница `security-doctor.md`.

### Изменено

- Версия инструмента обновлена до `1.6.0`.
- Версия каталога правил обновлена до `1.6.0`.
- HTML/Markdown-отчёты Security Doctor расширены до формата 2.0.
- Smoke-сценарий проверяет Security Doctor 2.0, allowlist секретов и security score.
- Sample reports пересобраны под формат 1.6.0.

### Проверено

- `go test ./...`.
- `go vet ./...`.
- `go build ./cmd/craftdoctor`.
- `scripts/smoke.sh`.
- `scripts/release.sh`.
- `craftdoctor security audit`.
- `craftdoctor security audit --format json`.

## 1.5.0

### Добавлено

- Performance Doctor 2.0 как расширенный слой анализа производительности.
- Импорт локальных spark/timings/profiler-отчётов из `reports`, `spark`, `timings`, `profiles` и `plugins/spark`.
- Извлечение TPS, MSPT avg, MSPT p95 и MSPT max из текстовых/HTML/JSON-отчётов.
- Флаги `--players` и `--target` для `craftdoctor performance scan`.
- Флаги `--players` и `--target` для полного `craftdoctor scan` и `craftdoctor ci`.
- Capacity-ориентиры по heap, `view-distance` и `simulation-distance` для `online`, `survival`, `rpg`, `minigames` и `custom`.
- Новые JSON-поля `performance.options`, `performance.capacity`, `performance.profiles`.
- Расширенные HTML/Markdown-блоки Performance Doctor 2.0.
- Новые правила `performance.profile.*` и `performance.capacity.*`.
- Новая Wiki-страница `performance-doctor.md`.

### Изменено

- Версия инструмента обновлена до `1.5.0`.
- Версия каталога правил обновлена до `1.5.0`.
- Smoke-сценарий проверяет импорт spark-отчёта и capacity-режим.
- Sample reports пересобраны под формат 1.5.0.

### Проверено

- `go test ./...`.
- `go vet ./...`.
- `go build ./cmd/craftdoctor`.
- `scripts/smoke.sh`.
- `scripts/release.sh`.
- `craftdoctor performance scan --players 80 --target rpg`.

## 1.4.0

### Добавлено

- Log Doctor 2.0 как расширенный слой анализа логов.
- Флаг `--last-run` для `craftdoctor logs analyze`.
- Флаг `--since` для `craftdoctor logs analyze`.
- Обнаружение сессий запуска сервера внутри `latest.log`.
- Выбор последней сессии запуска для отсечения старых ошибок.
- Fingerprint-группировка stack trace.
- Поле `main_root_cause` в JSON-отчёте.
- Поле `stack_traces` в JSON-отчёте.
- Поле `issue_types` в JSON-отчёте.
- Расширенные HTML/Markdown-блоки Log Doctor 2.0.
- Новые правила `logs.stacktrace.fingerprints.detected`, `logs.root_cause.primary`, `logs.exceptions.detected`.

### Изменено

- Версия инструмента обновлена до `1.4.0`.
- Версия каталога правил обновлена до `1.4.0`.
- Smoke-сценарий проверяет `logs analyze --last-run` и `logs analyze --since`.
- Sample reports пересобраны под формат 1.4.0.

### Проверено

- `go test ./...`.
- `go vet ./...`.
- `go build ./cmd/craftdoctor`.
- `scripts/smoke.sh`.
- `scripts/release.sh`.
- `craftdoctor logs analyze --last-run`.
- `craftdoctor logs analyze --since 24h`.

## 1.3.0

### Добавлено

- Plugin Intelligence 2.0 как расширенный слой Plugin Doctor.
- Команда `craftdoctor plugins graph <путь>`.
- Команда `craftdoctor plugins explain <путь> <плагин>`.
- Форматы графа зависимостей: `text`, `json`, `mermaid`, `dot`.
- Анализ `api-version`, `folia-supported`, `libraries`, `commands`, `permissions`, `bootstrapper`, `loader`.
- Анализ классов внутри jar: количество классов, наличие main class, package roots и shaded-библиотеки.
- Проверки отсутствующего/устаревшего `api-version`, отсутствующего main class и неподтверждённой Folia-совместимости.
- Потенциальные конфликты ролей плагинов и дублирующиеся классы.
- Новые правила `plugins.api_version.*`, `plugins.main_class.*`, `plugins.folia.*`, `plugins.shaded.*`, `plugins.classes.*`, `plugins.conflict.*`.

### Изменено

- Версия инструмента обновлена до `1.3.0`.
- Версия каталога правил обновлена до `1.3.0`.
- HTML/Markdown/JSON-отчёты расширены данными Plugin Intelligence.
- Smoke-сценарий проверяет `plugins graph` и `plugins explain`.

### Проверено

- `go test ./...`.
- `go vet ./...`.
- `go build ./cmd/craftdoctor`.
- `scripts/smoke.sh`.
- `scripts/release.sh`.
- `craftdoctor plugins graph`.
- `craftdoctor plugins explain`.

## 1.2.0

### Добавлено

- Rules Catalog 2.0 как рабочий JSON-каталог правил.
- Встроенный каталог правил через Go `embed`.
- Экспортируемый каталог `rules/catalog/*.json`.
- Команда `craftdoctor rules validate [путь]`.
- Текстовый и JSON-вывод проверки каталога правил.
- JSON Schema каталога правил: `schemas/rules-catalog.schema.json`.
- Проверка встроенного каталога, внешнего JSON-файла и директории JSON-модулей правил.
- Новая Wiki-страница `rules-catalog.md`.

### Изменено

- Версия инструмента обновлена до `1.2.0`.
- Версия каталога правил обновлена до `1.2.0`.
- Go-литерал встроенных правил заменён на JSON-каталог.
- Release-архивы теперь включают schema каталога правил и экспортируемый каталог правил.
- Smoke-сценарий проверяет `rules validate` и экспортируемый каталог.

### Проверено

- `go test ./...`.
- `go vet ./...`.
- `go build ./cmd/craftdoctor`.
- `scripts/smoke.sh`.
- `scripts/release.sh`.
- `craftdoctor rules validate`.

## 1.1.0

### Добавлено

- Config Doctor как отдельный рабочий слой.
- Команда `craftdoctor config audit <путь>`.
- Текстовый и JSON-вывод аудита конфигурации.
- Блок `config` в JSON-отчёте.
- Раздел Config Doctor в HTML/Markdown-отчётах.
- Анализ `server.properties`: дубли, неизвестные/устаревшие ключи, типы boolean/int, безопасные стартовые диапазоны.
- Анализ YAML-конфигов Bukkit/Spigot/Paper/Purpur: табуляции, пустые файлы, дубли ключей.
- Анализ `velocity.toml`: `bind`, `player-info-forwarding-mode`, `forwarding-secret-file`.
- Анализ BungeeCord/Waterfall `config.yml` и `ip_forward`.
- Проверка JSON-файлов `ops.json`, `whitelist.json`, ban-list/usercache.
- Новые правила `config.*`.

# История изменений

## 1.0.1

Patch-релиз для усиления релизной упаковки и подготовки проекта к публичной публикации на GitFlic.

Добавлено:

- безопасные ASCII-имена страниц GitFlic Wiki: `home.md`, `installation.md`, `cli.md`, `reports.md`, `ci-cd.md`;
- cleanup-скрипт `scripts/cleanup-legacy-wiki-names.sh` для удаления старых Wiki-файлов с повреждёнными именами после patch-обновления;
- проверка execute-битов и release-архивов в `scripts/smoke.sh`;
- проверка release-упаковки в `RELEASE_CHECKLIST.md`;
- файл `DELETED_FILES_1.0.1.txt` со списком файлов, которые нужно удалить при ручном применении patch-архива.

Изменено:

- версия инструмента обновлена до `1.0.1`;
- release-скрипт явно выставляет execute-биты для Linux/macOS-бинарников;
- Linux/macOS-релизы собираются в `tar.gz`, Windows-релиз — в `zip`;
- checksums формируются для финальных release-архивов, а не только для сырых бинарников;
- Wiki-заготовки больше не используют русские имена файлов, чтобы избежать проблем кодировки в ZIP/Android/Windows;
- README обновлён под hardening-релиз.

Проверено:

- `go test ./...`;
- `go vet ./...`;
- `go build ./cmd/craftdoctor`;
- `scripts/smoke.sh`;
- `scripts/release.sh`;
- проверка execute-битов release-бинарников;
- проверка наличия `tar.gz`/`zip` release-архивов и SHA256-файла.

## 1.0.0

Первый стабильный релиз CraftDoctor. Версия фиксирует CLI-контракт, стабильные exit codes, JSON Schema отчёта и CI-режим.

Добавлено:

- команда `craftdoctor ci <путь>` для CI/CD и quality gate;
- команда `craftdoctor schema report` для вывода стабильной JSON Schema отчёта;
- JSON-вывод `craftdoctor version --json`;
- файл `schemas/report.schema.json`;
- пакет `internal/schema`;
- стабильные exit codes: `0`, `1`, `2`, `64`;
- release-скрипт `scripts/release.sh`;
- цель `make release`;
- заготовки GitFlic Wiki в `.gitflic/wiki`;
- тесты CLI-слоя и JSON Schema.

Изменено:

- версия инструмента обновлена до `1.0.0`;
- версия каталога правил обновлена до `1.0.0`;
- README обновлён под стабильный релиз;
- `RELEASE_CHECKLIST.md` обновлён под release-сборку и CI;
- `rules/README.md` обновлён под стабильность правил;
- smoke-сценарий проверяет `ci`, `schema report`, `version --json` и exit code `2`;
- sample reports пересобраны;
- бинарник `bin/craftdoctor-linux-amd64` пересобран.

Проверено:

- `go test ./...`;
- `go build ./cmd/craftdoctor`;
- `make smoke`;
- `craftdoctor ci`;
- `craftdoctor schema report`;
- `craftdoctor version --json`;
- генерация HTML/Markdown/JSON-отчётов.

## 0.9.0

Минорный релиз с первым полноценным HTML Report 2.0.

Добавлено:

- обновлённый HTML-отчёт с dashboard-сводкой;
- общий score готовности проекта по шкале 0–100;
- sticky-навигация по разделам отчёта;
- фильтры находок по уровням `CRITICAL`, `DANGER`, `WARN`, `INFO`;
- сводка по категориям находок;
- dashboard Doctor-модулей: Plugin, Log, Performance, Proxy / Network, Security, Production;
- расширенный блок `Что исправить первым` в виде таблицы с ID, файлом и рекомендацией;
- блок команд для повторной проверки;
- collapsible-блоки для примеров логов;
- новые тесты HTML Report 2.0;
- smoke-проверки HTML Report 2.0.

Изменено:

- версия инструмента обновлена до `0.9.0`;
- версия каталога правил обновлена до `0.9.0`;
- README обновлён под HTML Report 2.0;
- sample HTML/Markdown/JSON отчёты пересобраны;
- бинарник `bin/craftdoctor-linux-amd64` пересобран.

Проверено:

- `go test ./...`;
- `go build ./cmd/craftdoctor`;
- `make smoke`;
- генерация HTML/Markdown/JSON-отчётов;
- наличие фильтров, score, dashboard и блока `Doctor-модули` в HTML-отчёте.

## 0.8.0

Минорный релиз с первым полноценным Proxy / Network Doctor.

Добавлено:

- команда `craftdoctor proxy scan <путь>`;
- текстовый и JSON-вывод proxy/network аудита;
- блок `proxy` в JSON-отчёте;
- блок `Proxy / Network Doctor` в HTML/Markdown-отчётах;
- анализ `velocity.toml`: `bind`, `player-info-forwarding-mode`, `forwarding-secret-file`, `[servers]`, `[forced-hosts]`;
- анализ BungeeCord/Waterfall `config.yml`: `ip_forward`, `online_mode`, `listeners`, `servers`;
- проверки backend `online-mode`, `server-ip`, публичного bind и внешних backend-адресов;
- построение простой карты сети proxy → backend;
- новые правила `proxy.velocity.*`, `proxy.bungee.*`, `proxy.backend.*`;
- тесты Proxy / Network Doctor.

Изменено:

- версия инструмента обновлена до `0.8.0`;
- версия каталога правил обновлена до `0.8.0`;
- README обновлён под Proxy / Network Doctor;
- `rules/README.md` обновлён proxy/network правилами;
- smoke-сценарий проверяет `proxy scan`, JSON-блок `proxy` и блок `Proxy / Network Doctor`.

Проверено:

- `go test ./...`;
- `go build ./cmd/craftdoctor`;
- `make smoke`;
- генерация HTML/Markdown/JSON-отчётов;
- команда `craftdoctor proxy scan`.

## 0.7.0

Минорный релиз с первым полноценным Production Doctor.

Добавлено:

- команда `craftdoctor production check <путь>`;
- текстовый и JSON-вывод production-ready аудита;
- блок `production` в JSON-отчёте;
- блок `Production Doctor` в HTML/Markdown-отчётах;
- проверки backup-кандидатов и свежести бэкапов;
- проверки service-файлов, restart policy и запуска от root;
- проверки logrotate/retention признаков;
- проверки скриптов запуска и execute-бита;
- проверки rollback/snapshot/release-кандидатов;
- проверки staging/test/preprod-кандидатов;
- проверки runtime-прав на корень сервера и `plugins`;
- новые правила `production.restart_policy.*`, `production.log_rotation.*`, `production.startup_scripts.*`, `production.rollback.*`, `production.staging.*`, `production.runtime.*`, `production.disk.*`;
- тесты Production Doctor.

Изменено:

- версия инструмента обновлена до `0.7.0`;
- версия каталога правил обновлена до `0.7.0`;
- README обновлён под Production Doctor;
- `rules/README.md` обновлён новыми production-ready правилами;
- smoke-сценарий проверяет `production check` и блок `Production Doctor`.

Проверено:

- `go test ./...`;
- `go build ./cmd/craftdoctor`;
- `make smoke`;
- генерация HTML/Markdown/JSON-отчётов;
- команда `craftdoctor production check`.

## 0.6.0

Минорный релиз с первым полноценным Security Doctor.

Добавлено:

- команда `craftdoctor security audit <путь>`;
- текстовый и JSON-вывод аудита безопасности;
- блок `security` в JSON-отчёте;
- блок `Security Doctor` в HTML/Markdown-отчётах;
- проверки `online-mode`, proxy forwarding, backend bind и forwarding secret;
- расширенные проверки RCON: включение, слабый пароль, rcon.port;
- проверки whitelist, command blocks, query и сетевых параметров;
- проверка world-writable прав на важные файлы и директории;
- поиск секретов в конфигурациях: Discord webhook, token/api key/secret, password;
- определение потенциальных security/permissions/rollback-плагинов по имени;
- новые правила `security.proxy.*`, `security.secrets.*`, `security.filesystem.*`, `security.plugins.*`, `security.query.enabled`, `security.whitelist.disabled_with_offline_mode`;
- тесты Security Doctor.

Изменено:

- версия инструмента обновлена до `0.6.0`;
- версия каталога правил обновлена до `0.6.0`;
- README обновлён под Security Doctor;
- `rules/README.md` обновлён новыми правилами безопасности;
- smoke-сценарий проверяет `security audit` и блок `Security Doctor`.

Проверено:

- `go test ./...`;
- `go build ./cmd/craftdoctor`;
- `make smoke`;
- генерация HTML/Markdown/JSON-отчётов;
- команда `craftdoctor security audit`.


## 0.5.0

Минорный релиз с первым полноценным Performance Doctor.

Добавлено:

- команда `craftdoctor performance scan <путь>`;
- текстовый и JSON-вывод анализа производительности;
- блок `performance` в JSON-отчёте;
- блок `Performance Doctor` в HTML/Markdown-отчётах;
- анализ JVM-флагов из `user_jvm_args.txt`, `jvm.args`, `server.args`, `start.sh`, `run.sh`, `start.bat`, `run.bat`;
- анализ `-Xms`, `-Xmx` и доли heap от RAM машины;
- анализ `view-distance`, `simulation-distance`, `entity-broadcast-range-percentage`, `max-tick-time`, `network-compression-threshold`;
- поиск признаков `keep-spawn-loaded` в Paper/Purpur-конфигах;
- учёт наличия spark и long tick предупреждений из Log Doctor;
- новые правила `performance.jvm.*`, `performance.metric.*`, `performance.logs.*`, `performance.spawn.keep_loaded`, `performance.profiling.spark_not_found`;
- тесты Performance Doctor.

Изменено:

- версия инструмента обновлена до `0.5.0`;
- версия каталога правил обновлена до `0.5.0`;
- README обновлён под Performance Doctor;
- `rules/README.md` обновлён новыми правилами производительности;
- smoke-сценарий проверяет `performance scan` и блок `Performance Doctor`.

Проверено:

- `go test ./...`;
- `go build ./cmd/craftdoctor`;
- `make smoke`;
- генерация HTML/Markdown/JSON-отчётов;
- команда `craftdoctor performance scan`.

## 0.4.0

Минорный релиз с первым полноценным Log Doctor.

Добавлено:

- команда `craftdoctor logs analyze <путь>`;
- текстовый и JSON-вывод анализа логов;
- расширенный блок `logs` в JSON-отчёте: `issues`, `root_causes`, `components`, `lines_analyzed`, `analyzed_file`;
- блок `Log Doctor` в HTML/Markdown-отчётах;
- группировка проблем логов по компонентам и типам;
- root-cause подсказки по отсутствующим зависимостям плагинов;
- root-cause подсказки по ошибкам загрузки/включения плагинов;
- root-cause подсказки по ошибкам БД, файловым правам, портам, Java/API и chunk/region;
- новые правила `plugins.load_failed`, `plugins.enable_failed`, `database.connection`, `filesystem.permissions`, `network.bind`, `java.compatibility`, `world.chunk_region`, `performance.long_tick`;
- тесты Log Doctor.

Изменено:

- версия инструмента обновлена до `0.4.0`;
- версия каталога правил обновлена до `0.4.0`;
- README обновлён под Log Doctor;
- `rules/README.md` обновлён новыми правилами логов;
- smoke-сценарий проверяет `logs analyze` и блок `Log Doctor`.

Проверено:

- `go test ./...`;
- `go build ./cmd/craftdoctor`;
- `make smoke`;
- генерация HTML/Markdown/JSON-отчётов;
- команда `craftdoctor logs analyze`.

## 1.0.0

Минорный релиз с первым полноценным Plugin Doctor.

Добавлено:

- команда `craftdoctor plugins audit <путь>`;
- текстовый и JSON-вывод аудита плагинов;
- блок `plugin_audit` в JSON-отчёте;
- сводка Plugin Doctor в HTML/Markdown-отчётах;
- категории плагинов: права, экономика, чат, защита территорий, производительность, миры, предметы/RPG, протокол, интеграции, база данных;
- граф зависимостей плагинов;
- анализ `softdepend`, `loadbefore`, `provides`, `authors`, `description`, `website`;
- поиск циклических обязательных зависимостей;
- поиск повреждённых jar-файлов;
- предупреждение о нескольких плагинах одной потенциально конфликтующей роли;
- правила `plugins.jar.invalid.*`, `plugins.dependency.cycle.*`, `plugins.category.multiple.*`;
- тесты Plugin Doctor.

Изменено:

- версия инструмента обновлена до `1.0.0`;
- таблица плагинов в HTML/Markdown-отчётах теперь показывает категории и softdepend;
- README обновлён под Plugin Doctor;
- `rules/README.md` обновлён новыми правилами плагинов;
- smoke-сценарий проверяет `plugins audit` и блок `plugin_audit`.

Проверено:

- `go test ./...`;
- `go build ./cmd/craftdoctor`;
- `make smoke`;
- генерация HTML/Markdown/JSON-отчётов;
- команда `craftdoctor plugins audit`.

## 1.0.0

Минорный релиз с первым рабочим rules engine.

Добавлено:

- встроенный каталог правил диагностики;
- пакет `internal/rules`;
- команда `craftdoctor rules list`;
- команда `craftdoctor rules explain <id>`;
- фильтры правил по `--severity` и `--category`;
- JSON-вывод для команд правил;
- поддержка дополнительного JSON-файла правил через `--rules-file`;
- поддержка `.craftdoctorignore` в корне сканируемого сервера;
- флаг `--ignore-file`;
- флаг `--no-ignore`;
- блок `rules` в JSON-отчёте;
- блок «Правила диагностики» в HTML/Markdown-отчётах;
- список подавленных находок в JSON-отчёте;
- пример `examples/craftdoctorignore.example`;
- тесты rules engine.

Изменено:

- версия инструмента обновлена до `1.0.0`;
- README обновлён под rules engine;
- `rules/README.md` обновлён под реальную реализацию правил;
- smoke-сценарий проверяет команды `rules list` и `rules explain`;
- smoke-сценарий проверяет наличие блока правил в JSON-отчёте;
- отчёт stderr показывает количество подавленных находок.

Проверено:

- `go test ./...`;
- `go build ./cmd/craftdoctor`;
- `make smoke`;
- генерация HTML/Markdown/JSON-отчётов.

## 0.1.1

Patch-релиз перед публикацией проекта на GitFlic.

Добавлено:

- `.editorconfig` для единых правил форматирования;
- `RELEASE_CHECKLIST.md` для подготовки GitFlic-релизов;
- `scripts/smoke.sh` для проверки сборки и базового сканирования;
- блок «Что исправить первым» в HTML и Markdown-отчётах;
- обновлённые примеры отчётов для версии 0.1.1.

Изменено:

- версия инструмента обновлена до `0.1.1`;
- README и служебные файлы синхронизированы с текущей версией;
- GitFlic issue template обновлён под 0.1.1.

Проверено:

- `go test ./...`;
- `go build ./cmd/craftdoctor`;
- smoke-сканирование тестовой директории Minecraft-сервера.

## 0.1.0

Первая рабочая MVP-версия CraftDoctor.

Добавлено:

- CLI-команда `scan`;
- команда `version`;
- команда `help`;
- HTML/Markdown/JSON-отчёты;
- анализ серверной машины;
- анализ Java;
- обнаружение Minecraft-ядра;
- чтение `server.properties`;
- проверка `eula.txt`;
- базовые security-проверки;
- анализ `.jar` плагинов через `plugin.yml` и `paper-plugin.yml`;
- поиск дублирующихся плагинов;
- поиск отсутствующих обязательных зависимостей;
- анализ `logs/latest.log`;
- обнаружение crash reports;
- production-ready проверки.
