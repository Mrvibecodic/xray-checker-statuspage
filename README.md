# xray-status (Go) — ветка `go-build`

[![Telegram — группа](https://img.shields.io/badge/Telegram-%D0%93%D1%80%D1%83%D0%BF%D0%BF%D0%B0-2CA5E0?style=for-the-badge&logo=telegram&logoColor=white)](https://t.me/+2lmP1yhxpCE3MDcy)
[![Telegram — чат](https://img.shields.io/badge/Telegram-%D0%A7%D0%B0%D1%82%20%D0%B8%20%D0%BF%D0%BE%D0%B4%D0%B4%D0%B5%D1%80%D0%B6%D0%BA%D0%B0-229ED9?style=for-the-badge&logo=telegram&logoColor=white)](https://t.me/+8BJQXYXYLqM4YWYy)

![xray-checker-statuspage](go-statuspage.jpg)

Go-порт статуспейджа поверх [xray-checker](https://github.com/kutovoys/xray-checker):
один статический бинарь — поднимает веб-страницу, опрашивает чекер и полностью
управляется Telegram-ботом (кнопками). Вид страницы — как в оригинале.

> Это ветка **`go-build`** (Go-версия). Ветка **`main`** — оригинальный
> Python-статуспейдж, остаётся для существующих пользователей. Go-образ
> публикуется под тегом **`:go-build`** и не задевает тех, кто тянет `:latest`.

## Установка (Docker)

1. Папка: `mkdir -p /opt/statuspage && cd /opt/statuspage`
2. Создай `docker-compose.yml` (впиши строки `ВПИШИ_…`):

```yaml
services:
  xray-checker:
    image: kutovoys/xray-checker
    container_name: xray-checker
    restart: unless-stopped
    network_mode: host
    depends_on: [statuspage]
    environment:
      SUBSCRIPTION_URL: "http://localhost:8081/sub"
      PROXY_CHECK_INTERVAL: "300"          # частота РЕАЛЬНЫХ проверок прокси (сек) — меняешь здесь
      SUBSCRIPTION_JSON_FORMAT: "true"     # балансиры-группы из XRAY_JSON проверяются по узлам; если панель JSON не отдаёт — работает как раньше

  statuspage:
    image: ghcr.io/mrvibecodic/xray-checker-statuspage:go-build
    container_name: statuspage
    restart: unless-stopped
    network_mode: host
    environment:
      BOT_TOKEN: "ВПИШИ_ТОКЕН_БОТА"        # от @BotFather; "" = только страница, без бота
      # TELEGRAM_PROXY: "socks5://user:password@127.0.0.1:1080" # (необяз.) SOCKS5 только для Telegram Bot API
      BOT_ADMIN_IDS: "ВПИШИ_СВОЙ_CHAT_ID"  # твой id от @userinfobot; несколько — через запятую
      # NOTIFY_CHAT_IDS: "-1001234567890"  # (необяз.) чаты/группы/каналы; топик форума — через двоеточие: -1001234567890:42 (прав на бота не дают)
      CHECKER_URL: "http://localhost:2112"
      TZ: "Europe/Moscow"
      # TITLE: ""     # необяз.; пусто = «Статус серверов»/«Server status» по LOCALE
      # SUBTITLE: ""  # необяз.; пусто = по LOCALE; заданные в боте тексты сохраняются
      # LOCALE: "ru"     # ru (по умолчанию) или en
      # WEB_PASSWORD: ""     # включает HTTP Basic Auth для веб-UI; пусто = без пароля
      # WEB_USER: ""     # необяз.; если задан — логин должен совпасть
      # --- за своим reverse-proxy (nginx/Caddy/Traefik): не занимает 80/443, без TLS ---
      # PORT: "8080"
      # TLS_MODE: "off"

      # --- Cloudflare Full (strict): свой Origin Certificate (файлы в том /data) ---
      # TLS_MODE: "file"
      # CERT_FILE: "/data/origin.pem"
      # KEY_FILE: "/data/origin.key"

      # --- Домен без Cloudflare-прокси (серт Let's Encrypt напрямую) ---
      # TLS_MODE: "letsencrypt"

      # --- PostgreSQL вместо встроенного SQLite ---
      # DB_DRIVER: "postgres"
      # DB_DSN: "postgres://user:pass@host:5432/db?sslmode=disable"
    volumes:
      - status-data:/data

volumes:
  status-data:
```

3. Запуск: `docker compose pull && docker compose up -d`
4. Открой `http://ТВОЙ_СЕРВЕР` — через минуту подтянутся данные. Дальше всё в боте (`/menu`).

Подписку и домен задаёшь в боте, в env их писать не нужно. Если Telegram
недоступен напрямую — `TELEGRAM_PROXY` (только для Bot API; спецсимволы в логине и
пароле URL-кодируй).

> Если xray-checker уже поднят отдельно — убери его блок, а у `statuspage` укажи
> `CHECKER_URL` на существующий чекер.

## Подписки чекера (из бота)

Раздел **🔌 Подписки**: добавить одну или несколько, посмотреть список, удалить
любую. Ввод — через запятую, по строке или каждую отдельным сообщением; текущие
**не затираются**. Бот объединяет все подписки и отдаёт чекеру одним списком на
loopback (`127.0.0.1:8081/sub`, наружу не торчит).

Серверы с одинаковой ремаркой из разных подписок считаются разными (как в
оригинальном чекере — по stableId) и показываются отдельными плитками; скрытие,
обслуживание и выключение работают по каждому отдельно.

После добавления/удаления **рестарт не нужен**: xray-checker сам перечитывает
`/sub` по своему интервалу обновления подписки (`SUBSCRIPTION_UPDATE_INTERVAL`,
по умолчанию ~5 мин) и затем протестит новые серверы. Нужен эффект сразу —
`docker compose restart xray-checker`.

### Балансиры (группы серверов)

Если панель отдаёт подписку в формате XRAY_JSON и в конфиге есть балансер из
нескольких узлов, чекер (с флагом `SUBSCRIPTION_JSON_FORMAT: "true"` из
compose-примера, версия ≥1.3.0) проверяет каждый узел отдельно, а статуспейдж
сворачивает их в **одну плитку**: онлайн, пока жив хотя бы один узел, с подписью
`N/M узлов`. `/sub` прозрачно пробрасывает панели заголовки чекера
(`User-Agent`, `X-Hwid`), так что JSON-формат работает и через несколько
подписок. В боте группа тоже везде **один сервер**: видимость, мьют,
обслуживание, инциденты и удаление применяются к группе целиком.

### 🩺 Диагностика подписки

Кнопка в разделе **🔌 Подписки**. Если серверы или балансир «не доехали» до
страницы — жми её вместо ручных curl: бот запрашивает каждую подписку ровно так,
как это делает чекер в json-режиме, показывает формат ответа панели (JSON или
base64-ссылки), число конфигов и балансир-группы, сравнивает с тем, что реально
видит чекер, и выносит вердикт — какое звено цепочки «панель → /sub → чекер»
порвано и что сделать (обновить чекер, включить флаг, поправить шаблон в
панели). Ссылки в отчёте маскируются до хоста, секреты в чат не попадают.

## Что умеет бот (всё кнопками, `/menu`)

- 🚀 **Веб-сервер** — старт/стоп/перезапуск; пока остановлен, порты 80/443
  свободны. HTTPS поднимается сам после 🌐 Задать домен (по умолчанию self-signed:
  за Cloudflare ставь SSL/TLS = **Full**, ошибки 525 не будет). Для **Full (strict)**
  — Origin Certificate через `CERT_FILE`/`KEY_FILE`; для домена **без** CF-прокси —
  `TLS_MODE=letsencrypt`.
- 🔌 **Подписки** и 🩺 **Диагностика** — см. выше.
- 🎨 **Вид страницы** — заголовок/подзаголовок/описание, тема, загрузка фавикона.
- ✏️ **Имена серверов** — своё имя сервера на странице (и в статусе/статистике
  бота), не трогая ремарку в панели; меню, алерты и подписка остаются на исходном
  имени. Флаг берётся из нового имени, если в нём есть страна. Сброс — кнопкой
  «Исходное имя».
- 🚨 **Инциденты** · 🛠 **Обслуживание** · 👁 **Видимость** · 📈 **Статистика** ·
  🧹 **База/очистка** (авто-удаление устаревших, удаление отсутствующих серверов —
  балансир-группа сносится целиком) · 📜 **Журнал**.
- ⚙️ **Настройки** — алерты (падение/восстановление, высокий пинг, глобальный
  сбой), заглушка уведомлений по отдельным серверам (🔕 Тихие серверы), время
  ежедневной сводки, 🧹 **автоудаление алертов** — сообщения «упал/восстановлен»
  и пинг-алерты сами удаляются из чатов спустя выбранный срок (6–48 ч; больше
  48 ч Telegram боту удалять не даёт).
- ⬆️ **Обновление** — скачать новую сборку `go-build` (по HTTPS, с проверкой
  SHA256) и перезапуститься.

Навигация правит одно сообщение; «🔄 Обновить статус» делает реальный опрос чекера.

## За своим reverse-proxy (nginx/Caddy/Traefik)

Раскомментируй в compose `PORT: "8080"` и `TLS_MODE: "off"` — сервис слушает
только `127.0.0.1:8080`, не трогает 80/443 и не поднимает TLS (этим займётся
прокси). Прокси форвардишь на `PORT` (`proxy_pass http://127.0.0.1:8080;`).

Порты можно взять любые свободные: на `PORT` смотрит `proxy_pass`; если сменил
`INTERNAL_PORT` (дефолт `8081`), **тот же** номер пропиши в `SUBSCRIPTION_URL` у
`xray-checker`.

> **502 Bad Gateway** от прокси — встроенный веб остановлен: 🚀 Веб-сервер → 🚀 Запустить.

Готовый пример конфига nginx бот отдаёт в 🚀 Веб-сервер → 🔧 Конфиг nginx:

```nginx
server {
    listen 80;
    server_name status.example.com;
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

TLS — обычным `certbot --nginx -d status.example.com`.

## Переменные окружения (основные)

| Переменная | По умолчанию | Назначение |
| --- | --- | --- |
| `BOT_TOKEN` | — | Токен бота от @BotFather (пусто — только страница) |
| `TELEGRAM_PROXY` | — | SOCKS5-прокси Telegram: `socks5://[user:password@]host:port` |
| `BOT_ADMIN_IDS` | — | Chat id админов через запятую |
| `NOTIFY_CHAT_IDS` | — | Чаты/группы/каналы для алертов; топик форума — `chatID:threadID` (прав на управление не дают) |
| `CHECKER_URL` | `http://xray-checker:2112` | Адрес xray-checker (в host-сети — `http://localhost:2112`) |
| `TZ` | `Europe/Moscow` | Часовой пояс серверных дат и сводки; задаётся при старте (JS-часы — в поясе устройства) |
| `TITLE` | ru: `Статус серверов`; en: `Server status` | Заголовок страницы (меняется в боте; явно заданное значение приоритетнее) |
| `SUBTITLE` | ru: `Доступность серверов в реальном времени`; en: `Real-time server availability` | Подзаголовок страницы (меняется в боте; явно заданное значение приоритетнее) |
| `PROXY_CHECK_INTERVAL` | `300` | **(на xray-checker)** частота проверок, сек; статуспейдж подстраивается сам |
| `TLS_MODE` | _(пусто → self-signed)_ | `off` / `file` (+`CERT_FILE`/`KEY_FILE`) / `letsencrypt` |
| `DB_DRIVER`, `DB_DSN` | `sqlite` | БД: `postgres` + DSN для PostgreSQL |
| `INTERNAL_PORT` | `8081` | Loopback-порт внутреннего `/sub` для чекера |

## Переустановка / полный сброс

```bash
docker compose down -v --remove-orphans   # -v стирает том: БД, secret.key, настройки и подписки
docker compose pull && docker compose up -d
```

Том `status-data` хранит БД, `secret.key` (шифрование подписок) и настройки — без
нужды его не удаляй, иначе сохранённые секреты перестанут расшифровываться.

## Сборка из исходников

Вся связка из текущего checkout (`docker-compose.build.yml` подменяет только образ
`statuspage` локальной сборкой; версия — через `VERSION=…`):

```bash
cp docker-compose.example.yml docker-compose.yml   # заполни BOT_TOKEN/BOT_ADMIN_IDS
docker compose -f docker-compose.yml -f docker-compose.build.yml up -d --build
```

Один бинарь без Docker:

```bash
CGO_ENABLED=0 go build -trimpath -o statuspage ./cmd/statuspage
```

Статический бинарь (SQLite через `modernc.org/sqlite`), кладётся в
`distroless/static`. Тесты: `go vet ./... && go test ./...`.
