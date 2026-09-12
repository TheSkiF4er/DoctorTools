# HTML Report 3.0

HTML Report 3.0 — основной человекочитаемый отчёт CraftDoctor. Он предназначен для владельцев Minecraft-серверов, технических администраторов, разработчиков плагинов и хостеров.

## Что появилось в 1.9.0

- dashboard-сводка проекта;
- score 0–100;
- score breakdown по штрафам CRITICAL / DANGER / WARN;
- risk matrix по категориям;
- поиск по находкам;
- фильтры по уровню серьёзности;
- группировка находок по категориям;
- блок экспортных команд;
- кнопка печати/сохранения в PDF средствами браузера;
- print-friendly режим.

## Генерация HTML-отчёта

```bash
craftdoctor scan /srv/minecraft --format html --output craftdoctor-report.html
```

## Передача отчёта

HTML-отчёт можно отправить владельцу проекта, хостеру или разработчику плагина как один файл. Для задач в GitFlic удобнее дополнительно приложить Markdown-отчёт:

```bash
craftdoctor scan /srv/minecraft --format markdown --output craftdoctor-report.md
```

Для CI/CD используется JSON:

```bash
craftdoctor scan /srv/minecraft --format json --output craftdoctor-report.json
```
