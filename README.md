# xray-status (Go) — ветка `go-build`

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

  statuspage:
    image: ghcr.io/mrvibecodic/xray-checker-statuspage:go-build
    container_name: statuspage
    restart: unless-stopped
    network_mode: host
    environment:
      # --- обязательное ---
      BOT_TOKEN: "ВПИШИ_ТОКЕН_БОТА"        # от @BotFather; "" = только страница, без бота
      BOT_ADMIN_IDS: "ВПИШИ_СВОЙ_CHAT_ID"  # твой id от @userinfobot; несколько — через запятую
      # NOTIFY_CHAT_IDS: "-1001234567890"  # (необяз.) доп. чаты/группы/каналы для алертов; неск. через запятую (прав на бота не дают)
      # --- базовое ---
      CHECKER_URL: "http://localhost:2112"
      TZ: "Europe/Moscow"
      TITLE: "Статус серверов"
      # --- За своим reverse-proxy (nginx/Caddy/Traefik): раскомментируй, и сервис
      #     НЕ займёт 80/443 и НЕ будет поднимать TLS сам — этим займётся твой прокси.
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

`BOT_TOKEN` — от @BotFather (пусто = только страница). `BOT_ADMIN_IDS` — твой id от
@userinfobot. `NOTIFY_CHAT_IDS` (необязательно) — доп. чаты/группы/каналы, куда бот
шлёт алерты и ежедневную сводку помимо админов; управлять ботом они не могут.
Подписку и домен задаёшь в боте, в env их писать не нужно.

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

## Что умеет бот (всё кнопками, `/menu`)

- 🚀 **Веб-сервер** — старт/стоп/перезапуск; пока остановлен, порты 80/443
  свободны. HTTPS поднимается сам после 🌐 Задать домен (по умолчанию self-signed:
  за Cloudflare ставь SSL/TLS = **Full**, ошибки 525 не будет). Для **Full (strict)**
  — Origin Certificate через `CERT_FILE`/`KEY_FILE`; для домена **без** CF-прокси —
  `TLS_MODE=letsencrypt`.
- 🔌 **Подписки** — см. выше.
- 🎨 **Вид страницы** — заголовок/подзаголовок/описание, тема, загрузка фавикона.
- 🚨 **Инциденты** · 🛠 **Обслуживание** · 👁 **Видимость** · 📈 **Статистика** ·
  🧹 **База/очистка** · 📜 **Журнал**.
- ⚙️ **Настройки** — алерты (падение/восстановление, высокий пинг, глобальный
  сбой), заглушка уведомлений по отдельным серверам (🔕 Тихие серверы), время
  ежедневной сводки.
- ⬆️ **Обновление** — скачать новую сборку `go-build` (по HTTPS, с проверкой
  SHA256) и перезапуститься.

Уведомления — админу в ЛС. Навигация правит одно сообщение; «🔄 Обновить статус»
делает реальный опрос чекера.

## За своим reverse-proxy (nginx/Caddy/Traefik)

Раскомментируй в compose `PORT: "8080"` и `TLS_MODE: "off"` — сервис слушает
только `127.0.0.1:8080`, не трогает 80/443 и не поднимает TLS (этим займётся
прокси). Прокси форвардишь на `PORT` (`proxy_pass http://127.0.0.1:8080;`).

**Смена портов.** Оба порта можно поставить любые свободные:
- `PORT` — публичный порт сервиса; на него же направляешь `proxy_pass` (поставил
  `9090` — и в конфиге прокси `9090`).
- `INTERNAL_PORT` — внутренний `/sub` для чекера (дефолт `8081`); если порт занят,
  смени его и **тот же** номер укажи в `SUBSCRIPTION_URL` у `xray-checker`
  (`http://localhost:<порт>/sub`) — они должны совпадать.

> **Если прокси отдаёт 502 Bad Gateway** — значит встроенный веб-сервер не
> запущен (прокси не достучался до `PORT`; в логах прокси — `connection refused`).
> Запусти его из бота: 🚀 Веб-сервер → 🚀 Запустить. По умолчанию веб включён.

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

## Частота проверок

Задаёт сам xray-checker (`PROXY_CHECK_INTERVAL`, дефолт `300` сек). Меняешь у
сервиса `xray-checker` и перезапускаешь только его: `docker compose up -d xray-checker`.
Статуспейдж узнаёт интервал у чекера (`/api/v1/config`) и опрашивает в тот же
такт; отдельной ручки в боте нет.

## Переменные окружения (основные)

| Переменная | По умолчанию | Назначение |
| --- | --- | --- |
| `BOT_TOKEN` | — | Токен бота от @BotFather (пусто — только страница) |
| `BOT_ADMIN_IDS` | — | Chat id админов через запятую |
| `NOTIFY_CHAT_IDS` | — | Доп. чаты/группы/каналы для алертов и сводки (прав на управление не дают) |
| `CHECKER_URL` | `http://xray-checker:2112` | Адрес xray-checker (в host-сети — `http://localhost:2112`) |
| `TZ` | `Europe/Moscow` | Часовой пояс (время и сводка) |
| `TITLE` | `Статус серверов` | Заголовок страницы (меняется в боте) |
| `PROXY_CHECK_INTERVAL` | `300` | **(на xray-checker)** частота проверок прокси, сек |
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

```bash
CGO_ENABLED=0 go build -trimpath -o statuspage ./cmd/statuspage
```

Статический бинарь (SQLite через `modernc.org/sqlite`), кладётся в
`distroless/static`. Тесты: `go vet ./... && go test ./...`.
