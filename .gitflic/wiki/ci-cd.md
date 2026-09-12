# CI/CD

CraftDoctor поддерживает локальные проверки и GitFlic CI.

## Локальный набор проверок

```bash
make lint
make test
make smoke
```

## Lint

```bash
make lint
```

Команда проверяет:

- форматирование Go-кода через `gofmt`;
- статический анализ `go vet ./...`;
- русскоязычность пользовательских Markdown/YAML/TXT-текстов.

## Smoke

```bash
make smoke
```

Smoke-сценарий создаёт тестовую директорию Minecraft-сервера и проверяет основные команды CraftDoctor:

- `scan`;
- `ci`;
- `rules list/explain/validate`;
- `config audit`;
- `plugins audit/graph/explain`;
- `logs analyze`;
- `performance scan`;
- `proxy scan`;
- `security audit`;
- `production check`;
- генерацию HTML/Markdown/JSON-отчётов;
- релизную упаковку;
- release manifest;
- SHA256-проверку артефактов.

## CI-режим CraftDoctor

```bash
craftdoctor ci /srv/minecraft --output craftdoctor-ci-report.json
```

Exit codes:

```text
0   Проверка пройдена
1   Ошибка выполнения
2   Quality gate не пройден
64  Ошибка использования CLI
```

## Quality gate

```bash
craftdoctor ci /srv/minecraft --max-critical 0 --max-danger 0 --max-warn 5
```

## GitFlic CI

В репозитории используется `.gitflic-ci.yml`.

Pipeline:

```text
check          make lint
test           make test
build          make build
smoke          make smoke
release-check  make release + make check-release
```

## Релизная сборка

```bash
make release
```

Подробности описаны на странице `release-automation.md`.
