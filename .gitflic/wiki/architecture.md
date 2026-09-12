# Архитектура DoctorTools

DoctorTools 3.0.1 использует общую архитектуру DoctorCore и набор стабильных продуктовых CLI-инструментов.

## Слои

```text
cmd/*                         входные точки CLI
internal/core                 общие модели, Analyzer API, Engine и writer отчётов
internal/version              реестр продуктов и версии
internal/products/stackdoctor полноценный StackDoctor
internal/products/depdoctor   полноценный DepDoctor
internal/products/logdoctor   полноценный LogDoctor
internal/products/configdoctor полноценный ConfigDoctor
internal/products/deploydoctor полноценный DeployDoctor
internal/app                  CraftDoctor
rules/*                       каталоги правил
schemas/*                     JSON Schema
```

## Product-подход

Каждый зрелый Doctor-инструмент должен иметь собственный пакет в `internal/products/<name>`, собственные команды, JSON-отчёт, unit-тесты и правила в `rules/<name>`.
