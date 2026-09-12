# CraftDoctor

CraftDoctor — стабильный продукт DoctorTools для диагностики Minecraft-серверных проектов.

Версия CraftDoctor в составе DoctorTools 3.0.1: `3.0.1`.

## Назначение

CraftDoctor выполняет read-only диагностику Minecraft-сервера: ядро, `server.properties`, Paper/Purpur/Bukkit/Spigot конфигурации, плагины, логи, производительность, proxy forwarding, безопасность, production-ready состояние и расширенный аудит миров/JVM/production-gate.

## Основные команды

```bash
craftdoctor scan /srv/minecraft
craftdoctor ci /srv/minecraft --max-critical 0 --max-danger 0
craftdoctor config audit /srv/minecraft
craftdoctor plugins audit /srv/minecraft
craftdoctor plugins graph /srv/minecraft --format mermaid
craftdoctor logs analyze /srv/minecraft --last-run
craftdoctor performance scan /srv/minecraft --players 80 --target rpg
craftdoctor proxy scan /srv/minecraft
craftdoctor proxy audit /srv/minecraft
craftdoctor security audit /srv/minecraft
craftdoctor production check /srv/minecraft
craftdoctor production gate /srv/minecraft
craftdoctor java flags /srv/minecraft
craftdoctor world audit /srv/minecraft
```

## Новое в 3.0.0

### Java flags audit

```bash
craftdoctor java flags /srv/minecraft --format json
```

Проверяет:

- наличие явных JVM-флагов запуска;
- `-Xmx` / `-Xms`;
- явный GC-профиль;
- частичную настройку G1GC;
- устаревшие флаги вроде CMS GC, `MaxPermSize`, `AggressiveOpts`;
- рекомендации для Java 17/21 и Paper/Purpur.

### World audit

```bash
craftdoctor world audit /srv/minecraft --format json
```

Проверяет:

- основной мир из `level-name`;
- nether/end директории;
- дополнительные директории с `level.dat` или `region`;
- `level.dat`, `session.lock`, `uid.dat`;
- количество region-файлов;
- размер region/entities;
- количество playerdata;
- datapacks.

### Production gate

```bash
craftdoctor production gate /srv/minecraft --format json
```

Проверяет Minecraft-specific блокеры перед production-запуском:

- EULA;
- heap/JVM-флаги;
- `online-mode=false` без proxy;
- наличие proxy forwarding;
- production startup через Docker/systemd/startup script;
- наличие backup-кандидатов.

## JSON-отчёт

В полном отчёте `craftdoctor scan --format json` добавлен блок:

```json
{
  "craft_advanced": {
    "status": "WARN",
    "worlds": [],
    "java_flag_checks": [],
    "gate_checks": [],
    "recommendations": []
  }
}
```

## Ограничения

CraftDoctor не изменяет файлы сервера, не скачивает зависимости, не исправляет конфигурации автоматически и не подключается к внешним API. Все проверки выполняются локально и read-only.
