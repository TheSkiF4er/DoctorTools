# APIDoctor

APIDoctor — product-инструмент DoctorTools для offline-first диагностики backend/API проектов.

- Версия: `1.0.1`.
- Релиз платформы: DoctorTools `2.7.0`.
- Статус: `product`.

## Назначение

APIDoctor проверяет, насколько backend/API проект готов к сопровождению, документации, безопасной публикации и production-деплою.

Основные зоны анализа:

- OpenAPI / Swagger спецификации;
- HTTP-маршруты и route declarations;
- auth/security middleware;
- CORS;
- rate limit / throttling;
- request validation;
- единый формат ошибок;
- pagination signals;
- health/readiness/liveness endpoints;
- базовый HTTP probe для URL.

## Команды

```bash
apidoctor scan ./backend
apidoctor openapi validate ./backend
apidoctor routes check ./backend
apidoctor security audit ./backend
apidoctor health check ./backend
apidoctor probe https://api.example.com --timeout 5s
```

Через общий CLI:

```bash
doctortools api scan ./backend
doctortools api security audit ./backend --json
```

## Отчёты

APIDoctor поддерживает текстовый вывод и JSON:

```bash
apidoctor scan ./backend --json
apidoctor scan ./backend --json --output reports/api.json
```

JSON-отчёт содержит:

- найденные API-файлы;
- найденные endpoints;
- findings;
- summary по severity;
- artifacts.

## Ограничения версии 1.0.1

APIDoctor 1.0.1 работает в offline-first режиме. Он не выполняет полноценное динамическое тестирование API, fuzzing, нагрузочное тестирование и authenticated runtime probing. HTTP probe ограничен проверкой доступности URL и типовых health endpoints.
