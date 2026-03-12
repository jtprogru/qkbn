# qkbn — Qwen-code Kanban Board

Локальный веб-сервер для визуализации активных задач Qwen-code в виде Kanban-доски.

## Что это такое

`qkbn` читает JSON-файлы с задачами из директории `~/.qwen/todos/` и отображает их в браузере в виде трёх колонок:

- **TODO (Pending)** — задачи, ожидающие выполнения
- **IN PROGRESS** — задачи в работе
- **DONE (Completed)** — завершённые задачи

Страница автоматически обновляется каждые 5 секунд.

![Пример](https://img.shields.io/badge/status-ready-green)

## Быстрый старт

```bash
# Запуск через go run
go run cmd/qkbn/main.go

# Откройте в браузере
# http://localhost:9090
```

## Установка

### Через Go

```bash
go install github.com/jtprogru/qkbn/cmd/qkbn@latest
qkbn
```

### Сборка из исходников

```bash
# Сборка бинарника
CGO_ENABLED=0 go build -o ./dist/qkbn cmd/qkbn/main.go

# Запуск
./dist/qkbn
```

### Через Task (рекомендуется)

```bash
# Установка Task: https://taskfile.dev/installation/

# Запуск
task run

# Сборка
task build

# Установка глобально
task install
```

## Использование

1. Запустите сервер:
   ```bash
   qkbn
   ```

2. Откройте в браузере: **http://localhost:9090**

3. Сервер автоматически читает последний изменённый JSON-файл из `~/.qwen/todos/`

### Формат задач

```json
{
  "todos": [
    {
      "content": "Реализовать функцию X",
      "activeForm": "Пишу код для модуля Y",
      "status": "in_progress"
    },
    {
      "content": "Написать тесты",
      "activeForm": "",
      "status": "pending"
    },
    {
      "content": "Обновить документацию",
      "activeForm": "Готово",
      "status": "completed"
    }
  ]
}
```

**Поля:**

| Поле | Описание |
|------|----------|
| `content` | Описание задачи |
| `activeForm` | Текущее действие (отображается с иконкой ⚡) |
| `status` | Статус: `pending`, `in_progress`, `completed` |

## Разработка

### Требования

- Go 1.25.0+
- [Task](https://taskfile.dev/) (опционально)
- [golangci-lint](https://golangci-lint.run/) (для линтинга)

### Основные команды

```bash
task run              # Запуск через go run
task build            # Сборка бинарника
task install          # Установка глобально
task fmt              # Форматирование кода
task lint             # Линтинг
task test             # Запуск тестов
task test:coverage    # Тесты с отчётом о покрытии
task clean            # Очистка артефактов
```

### Линтинг

```bash
golangci-lint run -v
```

## Технологии

- **Язык:** Go 1.25.0
- **Веб-сервер:** `net/http` (стандартная библиотека)
- **Шаблонизация:** `html/template`
- **Сборка:** Taskfile + GoReleaser

## Лицензия

MIT
