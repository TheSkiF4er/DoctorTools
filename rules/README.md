# Правила DoctorTools

Каталог `rules` содержит пользовательски видимые правила диагностики DoctorTools 3.x. Он нужен для документации, примеров, проверки схем и будущего подключения внешних каталогов правил.

Встроенный каталог CraftDoctor пока находится в `internal/rules/catalog` и встраивается в бинарник через Go embed. Внешний legacy-дубль `rules/catalog` удалён: он не является источником истины для DoctorTools 3.x.

## Актуальная структура

```text
rules/common/          общие правила логов, безопасности и системы
rules/craftdoctor/     Minecraft/server diagnostics
rules/stackdoctor/     full-stack diagnostics
rules/depdoctor/       dependency/supply-chain audit
rules/configdoctor/    Linux/Nginx/Docker/systemd config audit
rules/deploydoctor/    pre-deploy checks
rules/apidoctor/       backend/API diagnostics
rules/botdoctor/       Telegram/VK/Discord bot diagnostics
```

## Проверка правил

Встроенный каталог CraftDoctor:

```bash
craftdoctor rules validate --format json
```

Локальный пользовательский файл правил:

```bash
craftdoctor rules validate ./custom-rules.json --format json
```

Пример пользовательского файла:

```json
{
  "version": "local",
  "product": "craftdoctor",
  "profile": "minecraft",
  "rules": [
    {
      "id": "local.example.rule",
      "severity": "WARN",
      "category": "локальные правила",
      "title": "Пример локального правила",
      "description": "Описание локального правила.",
      "recommendation": "Рекомендация по исправлению."
    }
  ]
}
```

## .craftdoctorignore

Файл `.craftdoctorignore` кладётся в корень Minecraft-сервера.

Поддерживаются точные ID, wildcard, категории и уровни важности:

```text
security.spawn_protection.disabled
plugins.duplicate.*
plugins.dependency.cycle.*
category:логи
severity:INFO
```

Примеры находятся в `examples/craftdoctor/`.
