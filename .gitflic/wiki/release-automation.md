# Release automation

DoctorTools 3.0.1 использует проверяемый release pipeline.

## Проверки перед релизом

```bash
go test ./...
go vet ./...
make build-all
./scripts/smoke.sh
```

## Проверяемые версии

- DoctorTools 3.0.1 и CraftDoctor 3.0.1;
- StackDoctor 1.0.1;
- DepDoctor 1.0.1;
- ConfigDoctor 1.0.1;
- DeployDoctor 1.0.1;
- APIDoctor 1.0.1;
- BotDoctor 1.0.1;
- LogDoctor 1.0.1.

## Сборка релизных пакетов

```bash
./scripts/release.sh
```
