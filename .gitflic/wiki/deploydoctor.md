# DeployDoctor

**DeployDoctor** — product-инструмент DoctorTools для pre-deploy проверок проекта перед релизом, выкладкой или передачей архива клиенту.

## Статус

Версия: `1.0.1`  
Статус: `product`  
Платформа: `DoctorTools 2.4.0`

## Назначение

DeployDoctor проверяет, готов ли проект к передаче или production-деплою:

- наличие README, CHANGELOG, LICENSE, SECURITY и release checklist;
- наличие Makefile или CI-конфигурации;
- lock-файлы для Node.js/PHP/Go проектов;
- наличие безопасного `.env.example` и отсутствие реального `.env`;
- Dockerfile, `.dockerignore`, HEALTHCHECK, USER и pinned image tags;
- ZIP-архивы на `.env`, `.git`, `node_modules`, `vendor`, временные файлы и отсутствие инструкции.

## Команды

```bash
deploydoctor check .
deploydoctor release check .
deploydoctor ci check .
deploydoctor env check .
deploydoctor docker check .
deploydoctor archive check ./release.zip
```

Через единый CLI:

```bash
doctortools deploy check .
doctortools deploy archive check ./release.zip --json
```

## JSON-отчёт

```bash
deploydoctor check . --json
deploydoctor archive check ./release.zip --json --output deploy-report.json
```

## Практическое использование

Перед выдачей архива клиенту:

```bash
make build-all
deploydoctor check .
deploydoctor archive check ./release.zip
```

Перед публикацией release на GitFlic:

```bash
go test ./...
go vet ./...
deploydoctor release check . --json
```
