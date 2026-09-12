# DoctorTools Wiki

DoctorTools — монорепозиторий русскоязычных CLI-инструментов диагностики и аудита.

## Версия 3.0.1

DoctorTools 3.0.1 — release hygiene patch стабильной ветки 3.x. Релиз исправляет качество упаковки, права запуска бинарников и shell-скриптов, нормализует timestamps, обновляет схемы правил и выравнивает документацию после стабильного 3.0.0.

## Продукты

- CraftDoctor 3.0.1 — диагностика Minecraft-серверных проектов.
- StackDoctor 1.0.1 — full-stack диагностика.
- DepDoctor 1.0.1 — offline-аудит зависимостей и supply-chain рисков.
- ConfigDoctor 1.0.1 — аудит Linux/Nginx/Docker/systemd конфигураций.
- DeployDoctor 1.0.1 — pre-deploy проверки.
- APIDoctor 1.0.1 — backend/API диагностика.
- BotDoctor 1.0.1 — диагностика Telegram/VK/Discord ботов.
- LogDoctor 1.0.1 — анализ логов.

## Базовая проверка релиза

```bash
make build-all
./scripts/smoke.sh
./scripts/release.sh
```

После распаковки релизного архива `bin/*` и `scripts/*.sh` должны быть исполняемыми без ручного `chmod`.
