# Установка

## Из релиза GitFlic

1. Скачать бинарник под свою ОС из GitFlic Releases.
2. Проверить SHA256-хэш.
3. Поместить бинарник в директорию из `PATH` или запускать напрямую.

```bash
chmod +x craftdoctor-linux-amd64
./craftdoctor-linux-amd64 version
```

## Из исходников

Требуется Go 1.22+.

```bash
git clone <адрес-репозитория>
cd craftdoctor
go build -o craftdoctor ./cmd/craftdoctor
```
