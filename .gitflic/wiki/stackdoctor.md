# StackDoctor

StackDoctor — CLI-инструмент DoctorTools для offline-диагностики full-stack проектов.

## Статус

- Версия: `1.0.1`.
- Релиз платформы: DoctorTools `2.7.0`.
- Статус: `product`.

## Назначение

StackDoctor проверяет проект как целостную full-stack систему: структуру каталогов, frontend/backend компоненты, окружение, зависимости, API-документацию, Docker-ready состояние и production-readiness.

## Команды

```bash
stackdoctor scan .
stackdoctor structure check .
stackdoctor frontend scan .
stackdoctor backend scan .
stackdoctor env audit .
stackdoctor production check .
stackdoctor scan ./release.zip --json
```

Через общий CLI:

```bash
doctortools stack scan .
doctortools stack production check . --json
```

## Что проверяется

- наличие README, CHANGELOG, LICENSE, SECURITY и RELEASE_CHECKLIST;
- frontend/backend структура проекта;
- Node.js, PHP, Go, Python, Rust, Java и Docker компоненты;
- lock-файлы зависимостей;
- `.env` и `.env.example`;
- OpenAPI/Swagger schema;
- Dockerfile, `.dockerignore`, HEALTHCHECK, USER и latest image tag;
- тестовые и smoke-сценарии;
- CI/Makefile как признаки воспроизводимого релиза.

## Форматы отчёта

По умолчанию выводится текстовый отчёт. Для автоматизации используйте JSON:

```bash
stackdoctor scan . --json
stackdoctor production check . --output reports/stackdoctor.json
```

JSON-отчёт содержит:

- `components` — найденные компоненты проекта;
- `findings` — найденные проблемы и рекомендации;
- `summary` — количество critical/danger/warn/info;
- `artifacts` — служебная информация анализатора.

## Ограничения версии 1.0.1

StackDoctor 1.0.1 работает в offline-режиме. Он не обращается к внешним registry, CVE-базам и API. Глубокий API-probing и runtime-интеграции должны развиваться в APIDoctor и будущих версиях StackDoctor.
