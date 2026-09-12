# LogDoctor

LogDoctor — инструмент DoctorTools для анализа логов приложений, серверов и инфраструктурных сервисов.

## Статус

- Версия: `1.0.1`.
- Статус: `product`.
- Релиз платформы: DoctorTools `2.2.0`.

## Назначение

LogDoctor предназначен для первичной диагностики логов без внешних сервисов и без отправки данных в сеть. Инструмент ищет критичные и повторяющиеся признаки проблем, группирует находки и формирует отчёт.

Поддерживаемые расширения:

```text
.log
.txt
.out
.err
```

## Команды

```bash
logdoctor analyze ./logs
logdoctor summarize ./logs
logdoctor root-cause ./logs
logdoctor grep ./logs --level error
logdoctor analyze ./logs --json
logdoctor analyze ./logs --output reports/logdoctor.json
```

Через общий бинарник:

```bash
doctortools log analyze ./logs
doctortools log root-cause ./logs --json
doctortools log grep ./logs --level fatal
```

## Что проверяется

LogDoctor распознаёт:

- `fatal` и `critical` записи;
- `panic`;
- `exception`, `traceback`, `stack trace`;
- `error` / `level=error` / `[error]`;
- `warning` / `warn`;
- OOM и нехватку памяти;
- `permission denied`, `access denied`, `EACCES`;
- database errors: SQLSTATE, PostgreSQL, MySQL/MariaDB, SQLite, deadlock, connection errors;
- network errors: connection refused/reset, timeout, DNS, TLS/certificate errors.

## JSON-отчёт

JSON-отчёт содержит:

- общую сводку по severity;
- список найденных лог-файлов;
- количество обработанных строк;
- счётчики ошибок по типам;
- samples строк с номерами;
- группировку повторяющихся паттернов;
- список findings в общем формате DoctorTools.

## Ограничения версии 1.0.1

- CVE и внешние базы не используются.
- Анализ выполняется offline.
- Семантическое объединение stack trace пока базовое.
- `--since` зарезервирован для будущей версии и пока не применяется.

## Рекомендуемое использование

Перед релизом или после инцидента:

```bash
logdoctor root-cause ./logs --json --output reports/root-cause.json
```

Для быстрой проверки клиентского архива или серверного каталога:

```bash
logdoctor analyze .
```
