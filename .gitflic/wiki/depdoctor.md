# DepDoctor

**DepDoctor** — product-инструмент DoctorTools для offline-аудита зависимостей и supply-chain рисков.

## Статус

- Версия: `1.0.1`.
- Релиз платформы: DoctorTools `2.5.0`.
- Статус: `product`.

## Команды

```bash
depdoctor audit .
depdoctor lock check .
depdoctor scripts audit .
depdoctor licenses .
depdoctor outdated .
depdoctor supply-chain scan . --recursive
depdoctor audit . --json
doctortools dep audit .
```

## Проверки

- наличие lock-файлов;
- wildcard/latest/moving target версии;
- pre-release/dev версии;
- широкие диапазоны версий;
- Git/HTTP зависимости;
- локальные Go `replace`;
- Python-зависимости без `==`;
- подозрительные install/build scripts;
- `curl`, `wget`, pipe-to-shell, `powershell`, `base64 -d`, `chmod +x` паттерны;
- наличие явной лицензии в поддерживаемых манифестах.

DepDoctor 1.0.1 не обращается к CVE-базам и registry API. CVE/online-аудит должен развиваться отдельно.
