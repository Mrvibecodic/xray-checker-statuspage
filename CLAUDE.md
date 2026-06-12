# CLAUDE.md

Карта проекта для агента Claude (и для людей, кому нужна полная картина
механики). README рассчитан на пользователя-установщика; этот файл описывает
**как всё устроено внутри и почему так**.

## Что это

Кастомная страница статуса VPN/прокси-серверов. Исходно проект делался
поверх [xray-checker](https://github.com/kutovoys/xray-checker) (отсюда
название), но **в текущей версии xray-checker как зависимость удалён** —
statuspage сам тянет и парсит подписку, поэтому одна подписка автоматически
обновляется без рестартов внешних контейнеров.

**Текущая модель (probe-only, one-source):** живой статус «работает / не
работает» считается **только из данных region-пробников**, поставленных на
устройства пользователя в зоне блокировки (РФ). Список серверов и их имена
берутся из той же подписки, что и таргеты пробников — нет рассинхрона между
«что показывается на странице» и «что тестит агент».

```
PROBE_SUBSCRIPTION_URL  ──┐
                          │  тянет с TTL=PROBE_TARGETS_TTL_MIN, кеш в памяти
                          ▼
                    ┌──────────────┐
                    │  statuspage  │ ─── sync_subscription → current (UI)
                    │   (app.py)   │ ─── /api/probe/targets → агенту
                    └──────────────┘
                          ▲
                          │ POST /api/probe/report  {results | vpn:true}
                          │
                    ┌──────────────┐
                    │ probe-агент  │ TLS+VLESS handshake / xray-core
                    │ (на устр-ве) │ + geo-страж: если VPN → шлём vpn:true
                    └──────────────┘
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
| `docker-compose.example.yml` | Пример: `statuspage` (с `PROBE_SUBSCRIPTION_URL`). |
| `install.sh` | Установщик сервера: Docker, nginx, HTTPS, ADMIN_TOKEN. |
| `README.md` | Док для пользователя. |
| `CLAUDE.md` | Этот файл. |
| `.github/workflows/docker.yml` | Сборка образа в GHCR при пуше в `main`. |
| `probes/agent.py` | Probe-агент (xray-core + burst-проверка). Кросс-платформенный. |
| `probes/install-macos.sh` | Установщик агента на macOS (LaunchAgent + xray-core). |
| `probes/install-windows.ps1` | Установщик агента на Windows (Scheduled Task + xray.exe). |
| `probes/monitorvpn` / `probes/monitorvpn.ps1` | CLI управления агентом (macOS / Windows). |

`install.sh` ставит Docker/nginx/HTTPS, генерирует `ADMIN_TOKEN`, пишет
`docker-compose.yml` (узел тянет готовый образ из GHCR — без локальной сборки).

## app.py — устройство

### 1. Конфиг (env-переменные)

Читаются в начале `app.py`. Полный список с дефолтами — таблица в README.

**Сервер:**

| Переменная | Дефолт | Назначение |
|---|---|---|
| `POLL_INTERVAL` | `60` | период sync подписки → `current` |
| `DAYS` | `30` | ширина шкалы аптайма |
| `DB_PATH` | `/data/status.db` | путь к SQLite |
| `SERVER_HEADER` | `nginx` | маскировка заголовка `Server` |
| `ADMIN_TOKEN` | — | секрет админ-режима; пусто → выключен |
| `STALE_AFTER_HOURS` | `24` | порог удаления sid'ов, исчезнувших из подписки |

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

**Серверы (источник — подписка):**
- **`current`** — `(sid PK, name, online, latency, ts, seq)` — слепок
  серверов из подписки. `sid` = `"s" + sha256(host|port|sni|uuid)[:12]`,
  стабилен пока конфиг не меняется. `online`/`latency` колонки **не
  используются** (статус считается из probe-данных), но заполняются `(1, 0)`
  для backward-compat существующих БД. `ts` — момент последнего sync, `seq` —
  позиция строки в подписке (стабильный порядок на странице).
- **`daily`** / **`samples`** — legacy-таблицы от старой модели (когда
  xray-checker писал поминутные сэмплы). В новой версии не пополняются, но
  оставлены в схеме чтобы не ломать существующие установки.

**Пробники:**
- **`probes`** — `(probe_id PK, name, token_hash, created_at, last_seen,
  last_geo)`. **Несколько `probe_id` могут иметь одинаковый `name`** — это
  считается одним устройством (см. ниже про мердж).
- **`probe_samples`** — `(ts, probe_id, sid, ok, rtt, err)` — поминутные
  результаты handshake'ов. Чистится по `PROBE_SAMPLE_RETAIN_HOURS`
  (default = `(DAYS+1)*24`, чтобы хватало на 30-дневный график).
- **`probe_daily`** — `(day, probe_id, sid, up, down, lat_sum, lat_cnt,
  down_conf)`, PK `(day, probe_id, sid)`. Агрегаты по дням для 30-дневной
  шкалы аптайма.
- **`probe_vpn_samples`** — `(ts, probe_id, duration_sec)`, PK `(ts, probe_id)`.
  Когда агент детектит включённый VPN на устройстве (`geo != EXPECT_COUNTRY`),
  он шлёт «vpn-репорт» без results — сервер пишет одну запись. На графике
  такие интервалы показываются серой штриховкой («в это время был VPN, мы
  не тестили»). Подробнее — раздел «VPN-маркер» ниже.

**Служебное:**
- **`settings`** — k/v для рантайм-переключателей админ-режима (`autoclean`).
  Переживает рестарт.
- `hidden` — устаревшая, дропается при старте (`DROP TABLE IF EXISTS`).

### 3. Поток данных

```
┌──────────────────────┐  get_probe_targets   ┌──────────────────┐
│ PROBE_SUBSCRIPTION_  │ ─── кеш TTL=10мин ▶  │ in-memory cache  │
│ URL                  │                      │ _targets_cache   │
└──────────────────────┘                      └────────┬─────────┘
                                                       │
       ┌─────────── poll_once() ─── каждые POLL ───────┤
       ▼                                               ▼
┌──────────────┐                              ┌───────────────────┐
│  current     │   ← UI имена/флаги/порядок   │ /api/probe/targets│ → агенту
└──────────────┘                              └───────────────────┘

┌─────────────────┐                    ┌─────────────────────────┐
│ probe-агент     │ ──── каждые ─────▶ │ probe_samples /         │
│ (Mac/Linux/Win) │   INTERVAL         │ probe_daily /           │
│ POST /report    │   results | vpn    │ probe_vpn_samples       │
└─────────────────┘                    └─────────────┬───────────┘
                                                     │
                                                     ▼
                                      ┌────────────────────────┐
                                      │ build_summary          │
                                      │ → /api/summary         │
                                      └────────────────────────┘
```

### 4. `poll_once()` / `poller()` — sync подписки

`poller()` — фоновый поток (daemon), вызывает `poll_once()` каждые
`POLL_INTERVAL` секунд. `poll_once()`:

1. `get_probe_targets(force=False)` — тот же кеш, что у `/api/probe/targets`.
   Если кеш свежий — берёт из памяти, иначе тянет `PROBE_SUBSCRIPTION_URL`,
   парсит через `_parse_vless_line`. Подписка пустая/недоступна → return.
2. Для каждого таргета:
   - `sid = _sid_for_target(t)` — `"s" + sha256(host|port|sni|uuid)[:12]`.
   - `UPSERT` в `current(sid, name, online=1, latency=0, ts=now, seq=index)`.
3. **Авточистка устаревших sid'ов**: группируем `current` по `name`; удаляем
   группу, если ни один её sid не пришёл в свежей подписке И все члены
   старше `STALE_AFTER_HOURS`. Управление: env `STALE_AFTER_HOURS` (0 =
   мастер-выключатель) + тогл `autoclean` в settings.

### 5. Группировка `current` по имени хоста

Один логический хост в подписке может быть несколькими `sid`'ами (две строки
с одним `#name` — роутинг, или меняется один из ключей хеша → новый sid).
`build_summary` группирует `current` по сырому `name`. Текущий статус группы —
по самому свежему члену, агрегаты — наложением.

### 6. Автообновление подписки без рестартов

Это главная цель one-source-модели:
- Подписка кешируется in-memory на `PROBE_TARGETS_TTL_MIN` минут (default 10).
- По истечении кеша *любой* следующий запрос (poll_once или probe-агент) тянет
  подписку заново → `current` обновляется, агенты получают новые таргеты.
- `monitorvpn refresh` со стороны агента (или `?force=1` со стороны админа) —
  принудительный force-refresh кеша.
- Раньше xray-checker подписку без рестарта не обновлял, и `current` (имена
  для UI) отставал от probe-данных — теперь источник один, рассинхрона нет.

### 7. Пробники: основной источник данных в UI

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

### 8a. VPN-маркер (`save_probe_vpn`)

Когда агент детектит включённый VPN на устройстве (geo != `EXPECT_COUNTRY`),
пробы через туннель бессмысленны — получим ложное «всё работает». Цикл
пропускается, но **отчёт всё равно шлётся** с флагом `{vpn: true, results: []}`.
Сервер вызывает `save_probe_vpn(probe_id, geo)`:

1. Вставляет одну запись в `probe_vpn_samples (ts, probe_id, duration_sec)`.
2. `duration_sec` = `now - prev_ts`, где `prev_ts` — максимум `ts` среди
   всех записей этого пробника (обычные + vpn). Если разрыв > `2 ×
   PROBE_FRESH_MINUTES × 60` — пишем 60s (предполагаем «короткий пик»,
   а не «весь простой между пробуждениями был VPN»).
3. Обновляет `last_seen`/`last_geo`.
4. Чистит старые VPN-сэмплы по `PROBE_SAMPLE_RETAIN_HOURS`.

В `build_summary`/`_day_payload` VPN-сэмплы конвертятся в:
- `days[i].vpnMin` — минут VPN на каждый день шкалы (для оверлея на барах);
- `band.vpnMin30` — суммарно за окно;
- `vpnIntervals: [{start, end}]` в `/api/today`/`/api/day` — для серых полос
  на графике пинга. Соседние склеиваются, если зазор ≤ `2 × PROBE_FRESH_MINUTES`.

В UI:
- На барах дней — полупрозрачный серый rect (`.bandBars rect.vpn`), высота
  пропорциональна `vpnMin/1440`.
- На графике пинга — серые `<rect>` под линией (рисуются до красных
  полос сбоев, чтобы те были поверх).
- В подсказке дня — строка «на устройстве был VPN: X мин».

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
| `/api/admin/settings` | `{autoclean?}` | тоглы |
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
тоглом `autoclean`.

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

Однофайловый Python (только stdlib), **кросс-платформенный** — один и тот же
`agent.py` на macOS/Linux/Windows. Тестит конфиги **через xray-core** (бинарь
рядом: `~/.xrs-probe/xray[.exe]`), а не самописным TLS-handshake'ом. ENV:

| Переменная | Дефолт | |
|---|---|---|
| `STATUSPAGE_URL` | — | куда репортить |
| `PROBE_TOKEN` | — | выданный сервером при регистрации |
| `INTERVAL` | `60` | период цикла, сек |
| `TIMEOUT` | `10` | таймаут старта xray + одиночного запроса |
| `EXPECT_COUNTRY` | `RU` | ожидаемый ISO-код страны |
| `GEO_CHECK_URL` | `https://ifconfig.co/country-iso` | сервис geo-проверки |
| `XRAY_BIN` | — | явный путь к xray; иначе `~/.xrs-probe/xray[.exe]` → PATH |
| `PROBE_TEST_URL` | `http://cp.cloudflare.com/cdn-cgi/trace` | этап 1: туннель жив + ip |
| `PROBE_BULK_URL` | `https://www.youtube.com/` | этап 2 (burst): цель блокировок РФ |
| `PROBE_BURST_COUNT` | `30` | сколько параллельных коннектов в burst |
| `PROBE_BURST_OK_RATIO` | `0.5` | мин. доля успешных, иначе сервер «не работает» |
| `XRAY_DEBUG` | — | `1` → сохраняет последний xray-конфиг + не удаляет stdout/stderr |

Цикл (`one_cycle()`):

1. `fetch_geo()` — определяет страну. На macOS без `certifi` HTTPS-verify
   падает → fallback на unverified context (geo-check не критичен).
2. **VPN-страж**: если geo `!= EXPECT_COUNTRY` → шлёт `{vpn:true, results:[]}`
   (а не пропускает молча) — сервер пишет VPN-период, UI красит серым.
3. `fetch_targets()` — GET `/api/probe/targets`. Сервер отдаёт разобранный
   список с полями для xray-outbound: `{name, host, port, sni, uuid,
   security, pbk, sid, fp, flow, type, alpn, ...}`.
4. `_find_xray()` + `_check_xray_health()` — раз за процесс проверяет, что
   бинарь запускается (`xray version`). Нет/битый → отчёт с явной ошибкой по
   каждому таргету (не молчит).
5. `fetch_direct_ip()` — прямой IP без туннеля (baseline для сравнения).
6. Для каждого таргета — `xray_probe()`:
   - поднимает xray-core с inline-конфигом (`_build_xray_config`): HTTP-inbound
     на 127.0.0.1:rand-port + vless-outbound из таргета (`_stream_settings`
     собирает reality/tls/ws/grpc по полям);
   - **этап 1**: одиночный GET `PROBE_TEST_URL` через прокси → проверяет
     cloudflare-маркер `fl=` И что `ip=` отличается от `direct_ip` (иначе
     direct fallback);
   - **этап 2 (BURST, главный)**: `_burst_through_proxy()` открывает
     `PROBE_BURST_COUNT` параллельных коннектов к `PROBE_BULK_URL`. **Почему
     именно burst**: эмпирически (РФ, ТСПУ) DPI рубит REALITY-endpoint по
     ВСПЛЕСКУ хендшейков заблокированного fingerprint — одиночный коннект
     проходит даже на битом конфиге, пачка дохнет. Реальный клиент (Happ)
     открывает десятки коннектов разом. Если доля успешных <
     `PROBE_BURST_OK_RATIO` → сервер нерабочий;
   - xray-core тушится после каждого таргета.
7. `report()` — POST `/api/probe/report {geo, results:[...]}`.

**Почему xray-core, а не самописный handshake** (важная история, не повторять
ошибку): REALITY при невалидном клиенте делает fallback на легитимный dest —
TLS handshake проходит, UI показывает ложный «ok». Python `ssl` шлёт свой
fingerprint, не тот что в `fp=`. Только xray-core применяет ровно
fp/pbk/sid/flow из подписки. **И даже этого мало** — одиночный xray-коннект
проходит блок; ловит блок только burst (см. этап 2).

**Версия xray запинена на `v25.12.8`** в установщиках и `monitorvpn xray-update`
— 26.x ломает REALITY-конфиг (`empty "password"`). Override: env `XRAY_VERSION`.

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
- В `#adminbar` под статистикой — тогл «Авто-удаление устаревших записей»
  (`autoclean`).
- Кнопка темы — циклирует `light → dark → claude → claude-dark → light`.

## Жизненный цикл

`main()`: `init_db()` → фоновые потоки `ensure_fonts()` и `poller()` →
поднимает `ThreadingHTTPServer` на `0.0.0.0:PORT`.

## Типичные задачи и где их трогать

- **Новая env-настройка**: чтение в начале `app.py`, дефолт, документация в
  README-таблице, использование в нужной функции.
- **Новый рантайм-тогл админки**: ключ в `settings`, дефолт-функция (по
  аналогии с `autoclean_default`), отдача в GET `/api/admin/settings`,
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

## Как тестировать локально

Самый быстрый способ — поднять мок-HTTP, отдающий vless-подписку, и указать
его как `PROBE_SUBSCRIPTION_URL`:

```bash
# Мок-подписка на 127.0.0.1:18181 (два vless-сервера)
python3 -c "
import http.server, socketserver
SUB='vless://11111111-2222-3333-4444-555555555555@de1.example.com:443?security=reality&sni=microsoft.com#Germany\n'
class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        b=SUB.encode(); self.send_response(200); self.send_header('Content-Length',str(len(b))); self.end_headers(); self.wfile.write(b)
    def log_message(self,*a,**k): pass
socketserver.ThreadingTCPServer.allow_reuse_address=True
socketserver.ThreadingTCPServer(('127.0.0.1',18181),H).serve_forever()
" &

# statuspage
DB_PATH=/tmp/x/status.db PORT=18080 ADMIN_TOKEN=t POLL_INTERVAL=2 \
  PROBE_TARGETS_TTL_MIN=0 \
  PROBE_SUBSCRIPTION_URL=http://127.0.0.1:18181/sub python3 app.py
```

Для проверки пробника:
```bash
PROBE_TOKEN=$(curl -X POST -H "X-Admin-Token: t" -d '{"name":"test"}' \
  http://127.0.0.1:18080/api/admin/probes | python3 -c \
  "import sys,json;print(json.load(sys.stdin)['probe_token'])")
curl -X POST -H "X-Probe-Token: $PROBE_TOKEN" -H "Content-Type: application/json" \
  -d '{"geo":"ru","results":[{"name":"Switzerland","ok":true,"rtt":42}]}' \
  http://127.0.0.1:18080/api/probe/report
```
