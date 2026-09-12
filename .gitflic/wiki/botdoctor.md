# BotDoctor

BotDoctor — продукт DoctorTools для offline-диагностики Telegram/VK/Discord-ботов.

## Статус

Версия BotDoctor в составе DoctorTools 3.0.1: `1.0.1`, статус: `product`.

## Назначение

BotDoctor проверяет bot-проекты перед публикацией, переносом на VDS/BotHost или передачей клиенту. Инструмент не требует внешних API и работает по файлам проекта.

Основные зоны проверки:

- признаки Telegram/VK/Discord-интеграций;
- наличие `.env.example` и отсутствие реального `.env` в проекте;
- утечки Telegram/VK/Discord-токенов в исходниках;
- webhook, polling и long poll режимы;
- logging, rate limit и graceful shutdown;
- Dockerfile, systemd unit и runtime descriptor для BotHost/VDS;
- базовая готовность проекта к сопровождению.

## Команды

```bash
botdoctor scan ./bot
botdoctor telegram check ./bot
botdoctor discord check ./bot
botdoctor vk check ./bot
botdoctor env audit ./bot
botdoctor webhook check ./bot
botdoctor bothost check ./bot
botdoctor security audit ./bot
```

Через общий диспетчер:

```bash
doctortools bot scan ./bot
doctortools bot env audit ./bot --json
```

## JSON-отчёт

```bash
botdoctor scan ./bot --json
botdoctor security audit ./bot --output reports/botdoctor.json
```

Отчёт содержит:

- `tool`;
- `tool_version`;
- `target`;
- `command`;
- `summary`;
- `files`;
- `findings`;
- `artifacts`.

## Рекомендуемый минимум проекта

```text
README.md
.env.example
Dockerfile или systemd unit
Makefile или CI-конфигурация
логирование
rate limit/backoff
graceful shutdown
```

## Важно

Если BotDoctor нашёл токен в исходниках, токен нужно удалить из проекта, очистить историю Git и перевыпустить его на стороне платформы.
