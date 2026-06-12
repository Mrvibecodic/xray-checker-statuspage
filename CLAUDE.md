# CLAUDE.md

Карта проекта для агента Claude (и для людей, кому нужна полная картина
механики). README рассчитан на пользователя-установщика; этот файл описывает
**как всё устроено внутри и почему так**.

## Что это

Кастомная страница статуса VPN/прокси-серверов поверх
[xray-checker](https://github.com/kutovoys/xray-checker). xray-checker реально
тестит прокси через Xray и отдаёт API; этот проект — навигатор и хранилище
истории поверх него.

**Текущая модель (probe-only):** живой статус «работает / не работает» считается
**только из данных region-пробников**, поставленных на устройства пользователя
в зоне блокировки (РФ). xray-checker остался **только как поставщик списка
серверов** (имена, флаги, группы). Это сделано потому что облачный чекер не
видит блокировки по фингерпринту/SNI/IP, активные в РФ.

```
xray-checker        →  поставщик списка серверов (имена, флаги)
probe-агент         →  ходит на устройстве в РФ, делает TLS handshake к каждому
                       конфигу из подписки и репортит на сервер
statuspage          →  собирает имена от чекера + результаты от пробников,
                       рисует страницу, хранит историю в SQLite
```

## Стек и философия

- **Один файл — `app.py`.** Чистый Python 3.12, только stdlib
  (`http.server`, `sqlite3`, `urllib`, `gzip`, `threading`, `base64`, `ssl`).
  Никаких зависимостей, никакого фреймворка, нет `requirements.txt`. Это
  сознательно: `python:3.12-slim` + один файл, мгновенная сборка.
- **Фронтенд встроен в `app.py`** как строка `INDEX_HTML` (HTML+CSS+vanilla JS).
  Сборки фронта нет. Темизация — на CSS-переменных.
- **БД — SQLite** в одном файле (`/data/status.db`), режим WAL.
- **Пробник тоже одно-файловый Python** (`probes/agent.py`), только stdlib.

## Файлы репозитория

| Файл | Назначение |
|---|---|
| `app.py` | Весь backend + встроенный frontend. ~2000 строк. |
| `Dockerfile` | `python:3.12-slim` + `app.py`, запускает `python -u app.py`. |
| `docker-compose.example.yml` | Пример: `xray-checker` + `statuspage`. |
| `install.sh` | Установщик сервера: Docker, nginx, HTTPS, ADMIN_TOKEN. |
| `README.md` | Док для пользователя. |
| `CLAUDE.md` | Этот файл. |
| `.github/workflows/docker.yml` | Сборка образа в GHCR при пуше в `main`. |
| `probes/agent.py` | Probe-агент (TLS handshake, отчёты). |
| `probes/install-macos.sh` | Установщик агента на macOS (LaunchAgent). |
| `probes/monitorvpn` | CLI для управления агентом на macOS. |

`install.sh` ставит Docker/nginx/HTTPS, генерирует `ADMIN_TOKEN`, пишет
`docker-compose.yml` (узел тянет готовый образ из GHCR — без локальной сборки).

## app.py — устройство

### 1. Конфиг (env-переменные)

Читаются в начале `app.py`. Полный список с дефолтами — таблица в README.

**Сервер:**

| Переменная | Дефолт | Назначение |
|---|---|---|
| `CHECKER_URL` | `http://xray-checker:2112` | откуда брать имена серверов |
| `POLL_INTERVAL` | `60` | период опроса чекера (для синка имён) |
| `DAYS` | `30` | ширина шкалы аптайма |
| `SAMPLE_RETAIN_DAYS` | `DAYS+1` | retain старых (xray-checker) сэмплов |
| `DB_PATH` | `/data/status.db` | путь к SQLite |
| `SERVER_HEADER` | `nginx` | маскировка заголовка `Server` |
| `ADMIN_TOKEN` | — | секрет админ-режима; пусто → выключен |
| `STALE_AFTER_HOURS` | `24` | порог авто-удаления «призраков» |
| `GLOBAL_OUTAGE_RATIO` | `1.0` | отсев глобальных сбоев чекера |

**Пробники:**

| Переменная | Дефолт | Назначение |
|---|---|---|
| `PROBE_FRESH_MINUTES` | `10` | окно «свежести» проб для UI |
| `PROBE_SAMPLE_RETAIN_HOURS` | `(DAYS+1)*24` | retain probe-сэмплов |
| `PROBE_SUBSCRIPTION_URL` | — | URL подписки для пробников (сервер сам тянет/парсит и отдаёт агентам). Пусто → агенты получают 503. |
| `PROBE_SUBSCRIPTION_USER_AGENT` | `v2rayN/6.40` | UA при запросе подписки (некоторые панели смотрят на UA). |
| `PROBE_TARGETS_TTL_MIN` | `10` | TTL in-memory кеша таргетов. |

### 2. База данных

`init_db()` создаёт все таблицы (`CREATE TABLE IF NOT EXISTS`):

**Серверы (xray-checker — только как источник имён):**
- **`current`** — `(sid PK, name, online, latency, ts, seq)` — последнее
  известное состояние каждого `sid`. В новой модели `online`/`latency` из
  xray-checker НЕ показываются в UI, но поле `ts` используется авточисткой
  призраков и `seq` — для стабильного порядка серверов.
- **`daily`** — `(day, sid, up, down, lat_sum, lat_cnt, down_conf)` —
  агрегаты xray-checker'а. **В UI не показываются** — но в БД пишутся
  (на случай отката или переключения режима в будущем).
- **`samples`** — `(ts, sid, online, latency)` — поминутные точки xray-checker.
  Тоже хранятся, но в UI не используются.

**Пробники:**
- **`probes`** — `(probe_id PK, name, token_hash, created_at, last_seen,
  last_geo)`. **Несколько `probe_id` могут иметь одинаковый `name`** — это
  считается одним устройством (см. ниже про мердж).
- **`probe_samples`** — `(ts, probe_id, sid, ok, rtt, err)` — поминутные
  результаты TLS-handshake'ов. Чистится по `PROBE_SAMPLE_RETAIN_HOURS`
  (default = `(DAYS+1)*24`, чтобы хватало на 30-дневный график).
- **`probe_daily`** — `(day, probe_id, sid, up, down, lat_sum, lat_cnt,
  down_conf)`, PK `(day, probe_id, sid)`. Агрегаты по дням для 30-дневной
  шкалы аптайма.

**Служебное:**
- **`settings`** — k/v для рантайм-переключателей админ-режима
  (`autoclean`, `skip_global`). Переживает рестарт.
- `hidden` — устаревшая, дропается при старте (`DROP TABLE IF EXISTS`).

### 3. Поток данных

```
┌─────────────────┐ poll_once          ┌──────────────────┐
│ xray-checker    │ ──── каждые ────▶  │  current/daily/   │
│ /api/v1/proxies │   POLL_INTERVAL    │  samples          │
└─────────────────┘                    └────────┬──────────┘
                                                │ (используется
                                                │  только current
                                                │  для имён/групп)
                                                ▼
┌─────────────────┐                    ┌──────────────────┐
│ probe-агент     │ ──── каждые ────▶  │  probe_samples /  │
│ (Mac/Linux/...) │   INTERVAL         │  probe_daily      │
└─────────────────┘   POST /report     └────────┬──────────┘
                                                │
                                                ▼
                                        ┌──────────────────┐
                                        │  build_summary    │
                                        │  → /api/summary   │
                                        └──────────────────┘
```

### 4. `poll_once()` / `poller()`

`poller()` — фоновый поток (daemon), вызывает `poll_once()` каждые
`POLL_INTERVAL` секунд. `poll_once()`:

1. `fetch_proxies()` — GET `CHECKER_URL/api/v1/public/proxies`. Недоступен →
   исключение, цикл пропускается.
2. **Отсев глобального сбоя**: если доля офлайн-прокси ≥ `GLOBAL_OUTAGE_RATIO`
   и тогл `skip_global` в settings включён → `return`. Это спасает от
   ложных одинаковых аптаймов у всех серверов когда чекер падает.
3. Для каждого прокси с непустым `stableId`:
   - вычисляет `down_conf` («подтверждённый» простой — 2 подряд офлайн
     от того же sid),
   - `UPSERT` в `current`, инкремент в `daily`, `INSERT` в `samples`.
4. **Авточистка призраков**: удаляет sid'ы которые чекер перестал отдавать
   (см. ниже).
5. Чистка старья: `daily` старше `DAYS+1`, `samples` старше
   `SAMPLE_RETAIN_DAYS`.

### 5. Группировка `current` по имени хоста

Один логический хост в подписке может быть несколькими `stableId` (роутинг
внутри хоста, или смена парсера/конфига → новый id). `build_summary` группирует
`current` по сырому `name`. Текущий статус группы — по самому свежему члену,
агрегаты — наложением (сумма проверок всех членов).

### 6. Авточистка «призраков» (stale cleanup)

Когда у сервера меняется `stableId`, старая запись висит в `current` офлайн
навсегда. В конце `poll_once`:

- группируем `current` по `name`, удаляем из `current/daily/samples` только
  те группы, у которых **ВСЕ** члены старше `STALE_AFTER_HOURS`. Хост с
  альтернирующим роутингом не задевается.
- Управление: env `STALE_AFTER_HOURS` (0 = мастер-выключатель) + тогл
  `autoclean` в settings (UI админ-режима).

### 7. Отсев глобальных сбоев чекера

Если в одном опросе доля офлайн-прокси ≥ `GLOBAL_OUTAGE_RATIO` (default 1.0 =
когда офлайн все) и тогл `skip_global` включён — цикл не пишется в
`daily/samples`. Это значит ВСЕ серверы упали одновременно — практически
такого не бывает, это всегда артефакт чекера (рестарт, сетевой сбой). Раньше
без этого у географически разных серверов копился одинаковый ложный
даунтайм.

### 8. Пробники: основной источник данных в UI

**`probes` таблица** хранит регистрации. Каждый `probe_id` отдельный, но
**мердж по имени** — несколько `probe_id` с одинаковым `name` считаются одним
физическим устройством. Это нужно потому что повторная установка
`install-macos.sh` создавала новый `probe_id`, а старые висели → дубль.
Решение в две стороны:
- `install-macos.sh` шлёт `replace=true` при регистрации → старые с тем же
  именем удаляются (вместе с их samples/daily).
- В `build_summary` `regional[]` всегда группируется по имени — даже если
  дубли остались, на странице они сольются.

**`save_probe_report(probe_id, geo, results)`**:
1. Резолвит `name` каждого результата в canonical `sid` (мин seq группы).
2. Мерджит результаты с одинаковым sid: «любой ok → группа ok», rtt =
   min(ok-only).
3. Считает `down_conf` — 1 если этот результат офлайн И прошлый sample
   того же probe×sid тоже офлайн (≤ 2× окна свежести назад).
4. INSERT в `probe_samples`, UPSERT в `probe_daily`.
5. Обновляет `last_seen`/`last_geo` пробника.
6. Чистит `probe_samples`/`probe_daily` старше retain.

**В `build_summary`**: для каждой группы серверов и каждого уникального
имени пробника считает агрегаты по `probe_daily` (по всем `probe_id` с этим
именем, по всем `sid` группы). Поле каждой строки:

```jsonc
{
  "sid": "...", "name": "Россия", "cc": "ru",
  "online": true,       // зелёная точка если кто-то fresh ok
  "noData": false,      // если regional пуст
  "anyFresh": true,
  "latencyMs": 47,
  "uptime30": 99.5,
  "regional": [
    {
      "probe": "Mac-home",       // имя — идентификатор полосы
      "probeIds": ["p-...", "p-..."],  // все probe_id с этим именем
      "probe_id": "p-...",       // canonical (для legacy)
      "fresh": true,             // данные за последние PROBE_FRESH_MINUTES
      "online": true, "latencyMs": 47, "err": "",
      "uptime30": 99.5, "downMin30": 12,
      "days": [{ "date": "...", "uptime": 100.0, "downMin": 0, "hasData": true }, ...]
    }
  ]
}
```

### 9. HTTP-эндпоинты

`Handler(BaseHTTPRequestHandler)` + `ThreadingHTTPServer`. `_send()` умеет
gzip (для text/json/svg), gzip, ставит `Cache-Control`, `X-Robots-Tag`,
маскирует `Server`.

**Публичные GET:**

| Путь | Ответ |
|---|---|
| `/`, `/index.html` | HTML страницы |
| `/api/summary` | `build_summary()` — серверы + regional[] + totals |
| `/api/today?sid=&probeName=` | детализация сегодня для пробника (имя!) |
| `/api/day?sid=&date=&probeName=` | детализация конкретного дня |
| `/favicon.ico`, `/logo`, `/fonts/*` | бренд/шрифты |
| `/health` | `OK` |

**Админ GET (под `X-Admin-Token`):**

| Путь | Действие |
|---|---|
| `/api/admin/probes` | список пробников + `freshMinutes` |
| `/api/admin/probe-diag` | диагностика подписки (10+ User-Agent'ов) |

**Админ POST (под `X-Admin-Token`):**

| Путь | Body | Действие |
|---|---|---|
| `/api/admin/check` | — | валидация токена |
| `/api/admin/delete` | `{sid}` | удалить группу хоста по `sid` |
| `/api/admin/settings` | `{autoclean?, skipGlobal?}` | тоглы |
| `/api/admin/probes` | `{name, replace?}` | создать пробник; replace=true сносит существующих с этим именем |

**Probe POST (под `X-Probe-Token`):**

| Путь | Body | Действие |
|---|---|---|
| `/api/probe/report` | `{geo, results: [{name, ok, rtt, err}]}` | приём отчёта |
| `/api/probe/targets` | — | список таргетов (host/port/sni/name) для теста |

**DELETE (под `X-Admin-Token`):**
- `/api/admin/probes/<probe_id>` — удалить пробник.

Аутентификация:
- `ADMIN_TOKEN` — константно-временное сравнение через `_consteq`.
- `PROBE_TOKEN` — SHA-256-хеш в БД, lookup по индексу.

### 10. Фронтенд (строка `INDEX_HTML`)

Один HTML-шаблон с инлайн CSS и vanilla JS. Плейсхолдеры `__TITLE__`,
`__LOGO__` подставляются в `page_html()`.

**Главные DOM-узлы карточки сервера:**

```html
<div class="item">
  <div class="row">
    <div class="label">🏳️ Имя сервера ●</div>
    <div class="stat2">99.5% / 47ms · 30дн</div>
    <div class="chev">▾</div>
    <!-- if admin: × -->
  </div>
  <div class="bands">                            <!-- список полос -->
    <div class="band" _pid="Mac-home">          <!-- _pid = имя -->
      <div class="bandName">●  Mac-home</div>
      <svg class="bandBars">…30 rect…</svg>
      <div class="bandStat">99.5% / 47ms</div>
    </div>
    <!-- ещё полосы -->
  </div>
  <div class="panel">…график пинга при раскрытии…</div>
</div>
```

- `applyServer(item, s)` — обновляет общую шапку.
- `applyBands(item, s)` — синхронизирует `.bands` контейнер с `s.regional[]`.
  Идентификатор полосы — имя пробника (`r.probe`).
- `buildBand`/`updateBand` — рендер одной полосы. Клик по полосе или по
  бару дня раскрывает `.panel` с детализацией.
- `_probeQ(name)` строит `&probeName=...`. `openPanel/refreshPanel/loadDay`
  принимают имя, а не probe_id.

**Темы**: 4 штуки — `light`, `dark`, `claude` (warm cream + copper-orange),
`claude-dark` (warm dark + copper). Кнопка в шапке циклирует, выбор в
`localStorage`. Палитры — на CSS-переменных, переключаются через
`html[data-theme=...]`.

**Админ-режим**: иконка-замок → `prompt()` токена → `localStorage` →
появляется `×` у каждой строки (удаление группы) и панель `#adminbar` с
двумя тоглами (`autoclean`, `skip_global`).

### 11. Анти-фингерпринт

Чтобы инстансы было сложнее находить по шаблонным CSS-классам:

- `_uniquify()` при старте процесса заменяет «опознавательные» токены
  классов/id (список `_UNIQ_TOKENS`) на `c<random>token`. Префикс генерится
  из `os.urandom` один раз на запуск. Префикс уникален каждый рестарт
  контейнера.
- Заголовок `Server` маскируется под `SERVER_HEADER` (дефолт `nginx`).
- На странице `noindex, nofollow`.

**При добавлении нового класса/id, который не должен «палиться»**: добавь
его базовое имя в `_UNIQ_TOKENS`. Текущие токены: `tchart*`, `tscroll`,
`overall`, `pgrad`, `delbtn`, `lockon`, `lock`, `adminbar`, `adminrow`,
`adminlabel`, `actoggle`, `actogon`, `aclocked`, `bands`, `band`,
`bandName`, `bandPName`, `bandDot`, `bandDot{Ok,Bad,Stale}`, `bandBars`,
`bandStat`, `bandPct`, `bandSub`, `emptyBands`.

## Probe-агент (`probes/agent.py`)

Однофайловый Python (только stdlib). ENV:

| Переменная | Дефолт | |
|---|---|---|
| `STATUSPAGE_URL` | — | куда репортить |
| `PROBE_TOKEN` | — | выданный сервером при регистрации |
| `INTERVAL` | `60` | период цикла, сек |
| `TIMEOUT` | `10` | таймаут TLS handshake |
| `EXPECT_COUNTRY` | `RU` | ожидаемый ISO-код страны |
| `GEO_CHECK_URL` | `https://ifconfig.co/country-iso` | сервис для определения страны |

Цикл (`one_cycle()`):

1. `fetch_geo()` — определяет страну через `ifconfig.co/country-iso`.
   На macOS у Python без `certifi` HTTPS-verify падает → есть fallback на
   unverified context (geo-check не критичен для безопасности).
2. **VPN-страж**: если geo определилось и `!= EXPECT_COUNTRY` → **цикл
   пропускается, отчёт НЕ отправляется**. Это сделано чтобы не загрязнять
   данные когда на устройстве пробника случайно включён VPN. Если geo не
   определилось (timeout/ошибка ifconfig.co) — отчёт уходит, иначе при
   первом же сбое сети пробник перестал бы работать.
3. `fetch_targets()` — GET `STATUSPAGE_URL/api/probe/targets` с
   `X-Probe-Token`. Сервер сам тянет подписку (PROBE_SUBSCRIPTION_URL) и
   отдаёт разобранный список `[{name, host, port, sni}]`. Подписка с
   устройства пользователя НЕ светится.
4. Для каждого таргета — TCP-connect + TLS handshake к `host:port` с
   правильным SNI. Сертификат не верифицируется (REALITY использует
   невалидный — это норма). Если DPI обрывает на ClientHello — увидим
   `ConnectionResetError`. IP-блок — `TimeoutError`/`refused`.
5. `report()` — POST `STATUSPAGE_URL/api/probe/report`
   `{geo, results: [...]}`.

## CLI-команды

### На сервере (Linux)

```bash
sudo bash install.sh        # установка/обновление statuspage + nginx + HTTPS
                            # на "n" о перенастройке — pull нового образа +
                            # перезапуск + при необходимости добавить ADMIN_TOKEN
docker compose pull && docker compose up -d    # альтернатива
docker compose logs -f statuspage              # логи
docker exec statuspage env | grep PROBE        # проверить env
```

### На macOS (пробник)

```bash
# Установка
mkdir -p ~/xrs-probe && cd ~/xrs-probe
curl -fsSL https://raw.githubusercontent.com/Mrvibecodic/xray-checker-statuspage/main/probes/install-macos.sh -o install-macos.sh
bash install-macos.sh
# спросит: URL статус-страницы, ADMIN_TOKEN, имя пробника

# Управление (через CLI monitorvpn)
monitorvpn start       # запустить агента
monitorvpn stop        # остановить
monitorvpn restart     # перезапуск
monitorvpn status      # статус + последние строки лога
monitorvpn logs        # tail -f лога
monitorvpn refresh     # force-refresh подписки на сервере
monitorvpn delete      # полностью удалить
```

`monitorvpn` — bash-скрипт в `~/.xrs-probe/monitorvpn`, при установке через
`install-macos.sh` создаётся symlink в `/usr/local/bin/monitorvpn`. Если нет
прав на `/usr/local/bin/` — установщик подскажет точную `sudo ln -s` команду.

### На Windows (пробник)

```powershell
# Установка (нужен Python 3.10+, ставится через `winget install Python.Python.3.12`)
mkdir ~\xrs-probe; cd ~\xrs-probe
curl.exe -fsSL https://raw.githubusercontent.com/Mrvibecodic/xray-checker-statuspage/main/probes/install-windows.ps1 -o install-windows.ps1
.\install-windows.ps1

# Управление (после установки команда `monitorvpn` появится в PATH; перезайди
# в PowerShell, чтобы её увидеть).
monitorvpn start | stop | restart | status | logs | refresh | delete
```

Сделано через **Scheduled Task `XrayCheckerProbe`** с триггером «At log on» и
авто-перезапуском при падении. Конфиг (URL и `PROBE_TOKEN`) — в
`%USERPROFILE%\.xrs-probe\config.env`. Wrapper-скрипт `run-agent.ps1` читает
config, выставляет env-переменные и зовёт `python agent.py *>> agent.log`.
CMD-обёртка `%USERPROFILE%\.local\bin\monitorvpn.cmd` зовёт PowerShell-CLI
из `~/.xrs-probe/monitorvpn.ps1`; путь добавляется в User PATH установщиком.

### Обновление подписки

Сервер кеширует список таргетов на `PROBE_TARGETS_TTL_MIN` минут (default 10).
По истечении кеша при следующем запросе пробника сервер тянет подписку
заново. То есть если ты обновил подписку — пробники автоматически подхватят
изменения в течение TTL.

**Принудительное обновление** (если не хочешь ждать):
- Со стороны пробника: `monitorvpn refresh` (любая платформа). Шлёт
  `GET /api/probe/targets?force=1` с `X-Probe-Token` — сервер игнорирует
  кеш и тянет подписку немедленно.
- Со стороны админа: `GET /api/admin/probe-targets?force=1` с
  `X-Admin-Token` — то же, но без пробника (для CI или скриптов).

### В админ-режиме на странице

- Иконка-замок справа в шапке → `prompt()` ADMIN_TOKEN.
- × напротив каждой строки сервера — удалить группу хоста.
- В `#adminbar` под статистикой — два тогла:
  - «Авто-удаление устаревших записей» (`autoclean`)
  - «Игнорировать глобальные сбои чекера» (`skip_global`)
- Кнопка темы — циклирует `light → dark → claude → claude-dark → light`.

## Жизненный цикл

`main()`: `init_db()` → фоновые потоки `ensure_fonts()` и `poller()` →
поднимает `ThreadingHTTPServer` на `0.0.0.0:PORT`.

## Типичные задачи и где их трогать

- **Новая env-настройка**: чтение в начале `app.py`, дефолт, документация в
  README-таблице, использование в нужной функции.
- **Новый рантайм-тогл админки**: ключ в `settings`, дефолт-функция (по
  аналогии с `skip_global_default`), отдача в GET `/api/admin/settings`,
  приём в POST, второй `.adminrow` в HTML, токены в `_UNIQ_TOKENS`,
  JS через `postSetting(...)`.
- **Новая тема**: блок `html[data-theme="..."]{ --bg:…; … }` в CSS + ветка
  в JS-цикле темы (`NEXT`/`ICON`/`NAMES`).
- **Меняешь расчёт аптайма/простоя**: это `build_summary`. Проверяй на
  сидированной SQLite — заведи `current`/`probe_daily`/`probe_samples`
  руками и дёрни `/api/summary`.
- **Меняешь probe-агента**: проверь, что он умеет читать env с дефолтами;
  поправь `install-macos.sh` если меняется список env'ов в plist.
- **Меняешь UI**: проверь рендер при `noData=true` (нет пробников),
  при `fresh=false` (пробник молчит), при нескольких пробниках, при дубле
  имени (мердж).

## Как тестировать локально без чекера

```bash
DB_PATH=/tmp/x/status.db PORT=18080 ADMIN_TOKEN=t \
  CHECKER_URL=http://127.0.0.1:1 POLL_INTERVAL=3600 python3 app.py
```

Затем сидим данные руками через `sqlite3` и дёргаем `/api/summary`.
Для проверки опроса — поднять мок-HTTP, отдающий
`{"data":[{"stableId":...,"online":...}]}` на `/api/v1/public/proxies`, и
указать его как `CHECKER_URL` с маленьким `POLL_INTERVAL`.

Для проверки пробника:
```bash
PROBE_TOKEN=$(curl -X POST -H "X-Admin-Token: t" -d '{"name":"test"}' \
  http://127.0.0.1:18080/api/admin/probes | python3 -c \
  "import sys,json;print(json.load(sys.stdin)['probe_token'])")
curl -X POST -H "X-Probe-Token: $PROBE_TOKEN" -H "Content-Type: application/json" \
  -d '{"geo":"ru","results":[{"name":"Switzerland","ok":true,"rtt":42}]}' \
  http://127.0.0.1:18080/api/probe/report
```
