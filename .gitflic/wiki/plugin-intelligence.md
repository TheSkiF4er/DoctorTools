# Plugin Intelligence

Plugin Intelligence — расширенный слой анализа плагинов CraftDoctor в составе DoctorTools 3.x.

Он анализирует не только наличие `plugin.yml` / `paper-plugin.yml`, но и содержимое jar-файлов плагинов.

## Что проверяется

- `name`, `version`, `main`, `api-version`;
- `folia-supported`;
- `bootstrapper` и `loader` из `paper-plugin.yml`;
- `depend`, `softdepend`, `loadbefore`, `provides`;
- `libraries`;
- команды и права из `commands` / `permissions`;
- наличие main class внутри jar;
- количество `.class`-файлов;
- package roots;
- распространённые shaded-библиотеки;
- дубли классов между плагинами;
- потенциальные конфликты ролей;
- граф зависимостей плагинов.

## Команды

Аудит плагинов:

```bash
craftdoctor plugins audit /srv/minecraft
```

Граф зависимостей:

```bash
craftdoctor plugins graph /srv/minecraft
craftdoctor plugins graph /srv/minecraft --format mermaid
craftdoctor plugins graph /srv/minecraft --format dot
craftdoctor plugins graph /srv/minecraft --format json
```

Объяснение выбранного плагина:

```bash
craftdoctor plugins explain /srv/minecraft LuckPerms
craftdoctor plugins explain /srv/minecraft LuckPerms --format json
```

## Для чего это нужно

- быстро понять, какие плагины зависят друг от друга;
- найти отсутствующие зависимости;
- увидеть устаревшие `api-version`;
- проверить базовую Folia-совместимость;
- найти дублирующиеся jar и классы;
- понять, какой плагин содержит много shaded-библиотек;
- приложить отчёт к issue, MR или задаче на доработку сервера.
