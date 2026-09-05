# GophKeeper

GophKeeper — это безопасная клиент-серверная система для надёжного хранения приватной информации.

Проект написан на языке Go и предоставляет кроссплатформенное консольное приложение с терминальным пользовательским интерфейсом (TUI) на базе фреймворка Bubble Tea.

---

## Основные возможности

### Серверная часть
- Регистрация, аутентификация и авторизация пользователей (JWT).
- Безопасное хранение зашифрованных приватных данных.
- Выдача приватных данных авторизованному владельцу по запросу.

### Клиентская часть
- Кроссплатформенный CLI-клиент с интерактивным TUI (Windows, Linux, macOS).
- Аутентификация на удалённом сервере.
- Локальное кэширование и расшифровка данных "на лету".
- Просмотр, добавление, редактирование и удаление записей.
- Получение информации о версии и дате сборки бинарного файла.

### Поддерживаемые типы данных
1. Логины и пароли (логин, пароль, метаинформация).
2. Текстовые заметки (произвольный текст, метаинформация).
3. Банковские карты (номер, владелец, срок действия, CVV, метаинформация).
4. Бинарные данные (имя файла, MIME, бинарные файлы, метаинформация).

---

## Безопасность

- **Хеширование паролей**: Использование алгоритма `bcrypt` с настройками по умолчанию для хранения учётных данных на сервере.
- **Шифрование данных**: Полезная нагрузка (payload) шифруется перед отправкой на сервер.
- **Аутентификация**: Использование JSON Web Tokens (JWT) для защиты сессий.
- **Валидация**: Строгая валидация входных данных на стороне клиента перед шифрованием и отправкой (проверка длины, форматов карт, существования файлов).

---

## Установка и сборка

Для сборки проекта вам потребуется Go 1.26.1+.

### Сборка для текущей платформы сервера
```bash

go build -o gophkeeper-server ./cmd/server

go build \
  -ldflags="-X 'main.buildVersionServer=1.0.0' -X 'main.buildDateServer=$(date -u +%Y-%m-%dT%H:%M:%SZ)' -X 'main.buildCommitServer=Server GothKeeper'" \
  -o gophkeeper-server ./cmd/server
```

---

### Кроссплатформенная сборка клиента

Проект поддерживает сборку клиента под Windows, Linux и macOS. Используйте переменные окружения `GOOS` и `GOARCH`, а также флаги `-ldflags` для внедрения информации о версии, дате и коммите сборки:

```bash
# Linux (amd64)
GOOS=linux GOARCH=amd64 go build \
  -ldflags="-X 'main.buildVersionClient=1.0.0' -X 'main.buildDateClient=$(date -u +%Y-%m-%dT%H:%M:%SZ)' -X 'main.buildCommitClient=Client GothKeeper'" \
  -o gophkeeper-linux ./cmd/client

# macOS (arm64)
GOOS=darwin GOARCH=arm64 go build \
  -ldflags="-X 'main.buildVersionClient=1.0.0' -X 'main.buildDateClient=$(date -u +%Y-%m-%dT%H:%M:%SZ)' -X 'main.buildCommitClient=Client GothKeeper'" \
  -o gophkeeper-mac ./cmd/client

# Windows (amd64)
GOOS=windows GOARCH=amd64 go build \
  -ldflags="-X 'main.buildVersionClient=1.0.0' -X 'main.buildDateClient=$(date -u +%Y-%m-%dT%H:%M:%SZ)' -X 'main.buildCommitClient=Client GothKeeper'" \
  -o gophkeeper.exe ./cmd/client
```