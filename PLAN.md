# Teleprint — План разработки

## Context

Нужен микросервис-Telegram-бот для печати документов (PDF, JPG, PNG) на локальном сетевом принтере Brother HL-L2370DN через IPP. Бот приватный, с управлением доступом. UI — inline-кнопки для настройки параметров печати.

---

## 1. Стек технологий

| Задача | Библиотека | Обоснование |
|--------|-----------|-------------|
| Telegram Bot | `gopkg.in/telebot.v3` | Зрелый фреймворк, inline keyboards, middleware, proxy support |
| IPP печать | `github.com/phin1x/go-ipp` | Простой API, поддержка `io.Reader` (стриминг), атрибуты задания |
| mDNS discovery | `github.com/grandcat/zeroconf` | Стандарт де-факто, RFC 6762/6763, активно поддерживается |
| Image → PDF | `github.com/go-pdf/fpdf` | Поддерживаемый форк gofpdf, размещение изображений на A4 |
| Размеры изображений | `image` (stdlib) | `image.DecodeConfig` — читает только заголовки, без загрузки в RAM |
| Поворот изображений | `github.com/disintegration/imaging` | Pure Go (без CGO/libvips), простой API для Rotate90/180/270 |
| JSON хранилище | `encoding/json` (stdlib) | Для `users.json` — простой файл, atomic write |
| Логирование | `log/slog` (stdlib) | Встроенный structured logger (Go 1.21+) |

**Почему НЕ Maroto/bimg**: Maroto — оверкилл для одной картинки на листе. bimg требует libvips (CGO), что усложняет Docker-сборку.

---

## 2. Структура проекта

```
teleprint/
├── cmd/
│   ├── teleprint/
│   │   └── main.go              # Точка входа, wiring
│   └── testprint/
│       └── main.go              # Утилита для тестовой печати (шаг 1)
├── internal/
│   ├── bot/
│   │   ├── bot.go               # Инициализация telebot, роутинг
│   │   ├── handlers.go          # Обработчики: документ, фото, коллбэки
│   │   └── middleware.go        # Middleware: проверка доступа
│   ├── printer/
│   │   ├── discovery.go         # mDNS discovery (_ipp._tcp)
│   │   └── print.go             # Отправка задания на принтер через IPP
│   ├── converter/
│   │   └── image2pdf.go         # JPG/PNG → PDF (A4, авто-ориентация)
│   ├── state/
│   │   └── cache.go             # In-Memory кэш с TTL для UI-стейта
│   └── access/
│       └── users.go             # Управление users.json, /adduser, /deluser
├── .env.example                 # Шаблон переменных окружения
├── .gitignore
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── go.sum
```

---

## 3. In-Memory стейт (UI)

```go
type PrintJob struct {
    FileID      string    // Telegram file_id
    FileName    string    // Имя файла
    FileType    string    // "pdf", "jpg", "png"
    ChatID      int64     // Чат пользователя
    MessageID   int       // ID сообщения с кнопками (для редактирования)
    Copies      int       // Количество копий (1..99)
    FitMode     string    // "fit" (вписать) | "fill" (заполнить)
    Rotation    int       // 0, 90, 180, 270
    ImgWidth    int       // Оригинальные размеры (для фото)
    ImgHeight   int       // Оригинальные размеры (для фото)
    CreatedAt   time.Time // Для TTL
}
```

**Кэш**: `map[string]*PrintJob` (ключ = `fmt.Sprintf("%d:%d", chatID, messageID)`).
**TTL**: 15 минут. Фоновая горутина раз в минуту чистит просроченные записи.
**Конкурентность**: `sync.RWMutex`.

---

## 4. Пошаговый план разработки

### Шаг 1: Разведка принтера и тестовая печать ← НАЧИНАЕМ ЗДЕСЬ
- Обнаружение принтера в сети через mDNS (`_ipp._tcp`)
- Получение атрибутов принтера (имя, поддерживаемые форматы, возможности)
- Написать минимальный Go-скрипт `cmd/testprint/main.go` для тестовой печати
- Отправить тестовый PDF на принтер через IPP
- Убедиться что стриминг через `io.Reader` работает
- **Результат**: подтверждённая связка mDNS → IPP → принтер

### Шаг 2: Скелет проекта
- `go mod init`, структура директорий, `.env.example`, `.gitignore`
- `main.go`: загрузка конфига из `.env`, инициализация slog

### Шаг 3: Телеграм-бот (каркас)
- Подключение telebot с опциональным SOCKS5/HTTP прокси
- Middleware авторизации (проверка user ID)
- Команды `/start`, `/adduser`, `/deluser`
- `access/users.go`: загрузка/сохранение `users.json` с file lock

### Шаг 4: Обработка файлов + UI
- Хендлер на `OnDocument` (PDF) и `OnPhoto` (JPG/PNG)
- Скачивание через `bot.File()` → `io.Copy` в `/tmp`
- Для фото: `image.DecodeConfig` для получения размеров
- Создание `PrintJob` в кэше
- Отправка сообщения с inline-кнопками (копии, масштаб, поворот, печать, отмена)
- Обработка callback-запросов: обновление стейта → `EditReplyMarkup`

### Шаг 5: Конвертер изображений
- `image2pdf.go`: JPG/PNG → PDF
- Логика авто-ориентации: если фото landscape → лист landscape, и наоборот
- Ручной поворот на 90° (по кнопке)
- Режимы: "fit" (вписать с полями) / "fill" (заполнить, обрезая края)
- Результат: временный PDF файл (не в RAM)

### Шаг 6: Интеграция печати в бота
- Подключение модулей printer/discovery + printer/print из шага 1
- Полный цикл: файл → UI → конвертация → IPP → принтер
- При ошибке: алерт в чат, удаление задачи из кэша

### Шаг 7: Docker
- Multi-stage Dockerfile: `golang:1.24-alpine` → `alpine:3.21`
- `docker-compose.yml` с `network_mode: host`, `env_file: .env`, volume для `users.json`

### Шаг 8: Git + GitHub
- `git init`, `.gitignore`, первый коммит
- Запросить URL репозитория, `git remote add origin`, `git push`

---

## 5. Конфигурация (.env)

```env
TELEGRAM_TOKEN=         # Токен бота
ADMIN_ID=               # ID супер-админа
PROXY_URL=              # Опционально: socks5://host:port или http://host:port
PRINTER_NAME=           # Опционально: имя принтера (если не mDNS)
PRINTER_HOST=           # Опционально: IP принтера (фоллбэк без mDNS)
JOB_TTL_MINUTES=15      # TTL стейта в кэше
```

---

## 6. Поток данных

```
Пользователь → [файл/фото] → Bot
  → скачать во /tmp (io.Copy, стриминг)
  → определить тип (PDF / изображение)
  → если изображение: DecodeConfig → размеры
  → создать PrintJob в кэше
  → показать inline-кнопки

Пользователь → [настроил, нажал Печать]
  → если изображение: конвертировать в PDF (с поворотом/масштабом)
  → найти принтер (mDNS, кэш)
  → отправить PDF через IPP (стриминг)
  → удалить временные файлы
  → обновить сообщение: "Отправлено на печать"
```

---

## 7. Верификация

1. **Unit-тесты**: конвертер (image2pdf), кэш (TTL, concurrency), access (users.json)
2. **Интеграционный тест**: отправить фото/PDF боту → проверить inline-кнопки → нажать печать → проверить IPP-запрос
3. **Ручное тестирование**: `docker compose up`, отправить PDF и фото боту, убедиться что принтер печатает
4. **Edge cases**: файл > 20MB (лимит Telegram), принтер offline, невалидный формат

---

## Принятые решения

- **Язык UI**: только русский
- **Опции печати**: базовые (копии, масштаб, поворот) — без duplex и выбора бумаги
- **Формат бумаги**: только A4
- **Репозиторий**: `teleprint`
- **Принтер**: доступен в сети, можно тестировать сразу
- **Макс. копий**: 99
