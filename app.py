import base64
import gzip
import json
import os
import re
import sqlite3
import threading
import time
import urllib.request
from urllib.parse import urlparse, parse_qs
from datetime import datetime, timedelta, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

try:
    from zoneinfo import ZoneInfo
except Exception:
    ZoneInfo = None

POLL_INTERVAL = int(os.environ.get("POLL_INTERVAL", "60"))
DAYS = int(os.environ.get("DAYS", "30"))
DB_PATH = os.environ.get("DB_PATH", "/data/status.db")
PORT = int(os.environ.get("PORT", "8080"))
TITLE = os.environ.get("TITLE", "Статус серверов")
SUBTITLE = os.environ.get("SUBTITLE", "Доступность серверов в реальном времени")
TZ_NAME = os.environ.get("TZ", "Europe/Moscow")
SERVER_HEADER = os.environ.get("SERVER_HEADER", "nginx")
ADMIN_TOKEN = os.environ.get("ADMIN_TOKEN", "").strip()
# Сколько часов держать запись в `current`, если сервер перестал отдаваться в
# подписке. 0 — авточистку выключить (старые sid'ы будут висеть в БД, пока
# не удалят руками через × в админ-режиме).
STALE_AFTER_HOURS = int(os.environ.get("STALE_AFTER_HOURS", "24"))
STATIC_CACHE = "public, max-age=31536000, immutable"
NO_CACHE = "no-cache, no-store, must-revalidate"

RU_MONTHS = ["", "янв", "фев", "мар", "апр", "май", "июн",
             "июл", "авг", "сен", "окт", "ноя", "дек"]

FONT_DIR = os.path.join(os.path.dirname(DB_PATH) or ".", "fonts")
FONT_BASE = "https://cdn.jsdelivr.net/npm/@fontsource/inter@5/files/"
FONT_FILES = [
    "inter-latin-400-normal.woff2", "inter-latin-500-normal.woff2",
    "inter-latin-600-normal.woff2", "inter-cyrillic-400-normal.woff2",
    "inter-cyrillic-500-normal.woff2", "inter-cyrillic-600-normal.woff2",
]


def ensure_fonts():
    try:
        os.makedirs(FONT_DIR, exist_ok=True)
    except Exception:
        return
    for fn in FONT_FILES:
        fp = os.path.join(FONT_DIR, fn)
        try:
            if os.path.isfile(fp) and os.path.getsize(fp) > 0:
                continue
            req = urllib.request.Request(FONT_BASE + fn, headers={"User-Agent": "xray-status"})
            with urllib.request.urlopen(req, timeout=20) as r:
                data = r.read()
            with open(fp, "wb") as f:
                f.write(data)
            print("font ok:", fn, flush=True)
        except Exception as e:
            print("font fail:", fn, e, flush=True)

BRAND_DIRS = [d for d in [os.environ.get("ASSETS_DIR"),
                          os.path.dirname(DB_PATH) or ".", "/data", "/app", "."] if d]
IMG_EXT = {".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
           ".webp": "image/webp", ".ico": "image/x-icon", ".svg": "image/svg+xml",
           ".gif": "image/gif"}
DEFAULT_LOGO_SVG = ('<svg width="21" height="21" viewBox="0 0 24 24" fill="none" '
    'stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">'
    '<path d="M12 3l7 4v5c0 4.5-3 7.5-7 9-4-1.5-7-4.5-7-9V7z"/><path d="M9 12l2 2 4-4"/></svg>')


def find_brand_image():
    for d in BRAND_DIRS:
        try:
            for fn in sorted(os.listdir(d)):
                low = fn.lower()
                if low.startswith("favicon") and os.path.splitext(low)[1] in IMG_EXT:
                    return os.path.join(d, fn)
        except Exception:
            pass
    return None

COUNTRY_KEYWORDS = [
    ("netherland", "nl"), ("нидерланд", "nl"), ("holland", "nl"), ("amsterdam", "nl"),
    ("germany", "de"), ("герман", "de"), ("frankfurt", "de"), ("deutsch", "de"),
    ("finland", "fi"), ("финлянд", "fi"), ("helsinki", "fi"),
    ("united states", "us"), ("usa", "us"), ("сша", "us"), ("america", "us"),
    ("new york", "us"), ("los angeles", "us"), ("miami", "us"), ("dallas", "us"), ("seattle", "us"),
    ("united kingdom", "gb"), ("britain", "gb"), ("england", "gb"), ("london", "gb"),
    ("великобритан", "gb"), ("англия", "gb"),
    ("france", "fr"), ("франц", "fr"), ("paris", "fr"), ("marseille", "fr"),
    ("japan", "jp"), ("япон", "jp"), ("tokyo", "jp"), ("osaka", "jp"),
    ("singapore", "sg"), ("сингапур", "sg"),
    ("turkey", "tr"), ("турц", "tr"), ("istanbul", "tr"), ("стамбул", "tr"),
    ("russia", "ru"), ("росси", "ru"), ("moscow", "ru"), ("москва", "ru"), ("питер", "ru"),
    ("poland", "pl"), ("польш", "pl"), ("warsaw", "pl"),
    ("sweden", "se"), ("швец", "se"), ("stockholm", "se"),
    ("switzerland", "ch"), ("швейцар", "ch"), ("zurich", "ch"),
    ("canada", "ca"), ("канад", "ca"), ("toronto", "ca"),
    ("italy", "it"), ("italia", "it"), ("итал", "it"), ("milan", "it"), ("milano", "it"),
    ("rome", "it"), ("roma", "it"), ("naples", "it"), ("napoli", "it"), ("неапол", "it"),
    ("turin", "it"), ("torino", "it"), ("турин", "it"), ("venice", "it"), ("venezia", "it"), ("венеци", "it"),
    ("spain", "es"), ("испан", "es"), ("madrid", "es"),
    ("hong kong", "hk"), ("гонконг", "hk"),
    ("korea", "kr"), ("корея", "kr"), ("seoul", "kr"),
    ("india", "in"), ("инди", "in"), ("mumbai", "in"),
    ("austria", "at"), ("австри", "at"), ("vienna", "at"),
    ("norway", "no"), ("норвег", "no"), ("oslo", "no"),
    ("denmark", "dk"), ("дани", "dk"),
    ("ireland", "ie"), ("ирланд", "ie"), ("dublin", "ie"),
    ("czech", "cz"), ("чех", "cz"), ("prague", "cz"),
    ("ukraine", "ua"), ("украин", "ua"), ("kyiv", "ua"), ("kiev", "ua"),
    ("emirates", "ae"), ("dubai", "ae"), ("оаэ", "ae"), ("uae", "ae"),
    ("israel", "il"), ("израил", "il"),
    ("brazil", "br"), ("бразил", "br"),
    ("australia", "au"), ("австрал", "au"), ("sydney", "au"),
    ("china", "cn"), ("китай", "cn"),
    ("hungary", "hu"), ("венгр", "hu"),
    ("romania", "ro"), ("румын", "ro"),
    ("bulgaria", "bg"), ("болгар", "bg"),
    ("latvia", "lv"), ("латви", "lv"), ("riga", "lv"),
    ("lithuania", "lt"), ("литва", "lt"),
    ("estonia", "ee"), ("эстони", "ee"),
    ("kazakhstan", "kz"), ("казахстан", "kz"),
    ("georgia", "ge"), ("груз", "ge"),
    ("armenia", "am"), ("армени", "am"),
    ("serbia", "rs"), ("серб", "rs"),
    ("greece", "gr"), ("греци", "gr"),
    ("portugal", "pt"), ("португал", "pt"),
    ("belgium", "be"), ("бельги", "be"),
    ("mexico", "mx"), ("мексик", "mx"),
    ("argentina", "ar"), ("аргентин", "ar"),
]

_lock = threading.Lock()


def detect_country(name):
    n = (name or "").lower()
    for kw, cc in COUNTRY_KEYWORDS:
        if kw in n:
            return cc
    return ""


def display_name(name, cc):
    if not name:
        return name
    s = re.sub("[\U0001F1E6-\U0001F1FF]", "", name).strip()
    if cc:
        m = re.match(r'^[A-Za-zА-Яа-яЁё]{2,3}[\s\-_|.]+(.+)$', s)
        if m:
            s = m.group(1).strip()
    return s or name


def tz():
    if ZoneInfo is not None:
        try:
            return ZoneInfo(TZ_NAME)
        except Exception:
            pass
    return timezone.utc


def conn():
    c = sqlite3.connect(DB_PATH, check_same_thread=False)
    c.execute("PRAGMA journal_mode=WAL")
    return c


def init_db():
    with _lock, conn() as c:
        c.execute("""CREATE TABLE IF NOT EXISTS current(
            sid TEXT PRIMARY KEY, name TEXT, online INTEGER,
            latency INTEGER, ts INTEGER, seq INTEGER)""")
        c.execute("""CREATE TABLE IF NOT EXISTS daily(
            day TEXT, sid TEXT, up INTEGER DEFAULT 0, down INTEGER DEFAULT 0,
            lat_sum INTEGER DEFAULT 0, lat_cnt INTEGER DEFAULT 0,
            PRIMARY KEY(day, sid))""")
        try:
            c.execute("ALTER TABLE daily ADD COLUMN down_conf INTEGER DEFAULT 0")
        except Exception:
            pass
        c.execute("""CREATE TABLE IF NOT EXISTS samples(
            ts INTEGER, sid TEXT, online INTEGER, latency INTEGER)""")
        c.execute("CREATE INDEX IF NOT EXISTS idx_samples ON samples(sid, ts)")
        c.execute("DROP TABLE IF EXISTS hidden")
        c.execute("""CREATE TABLE IF NOT EXISTS settings(
            k TEXT PRIMARY KEY, v TEXT)""")
        # Региональные пробники: устройства в зоне блокировки (РФ), которые тестят
        # прокси-конфиги локальным TLS-handshake'ом — чтобы детектить ban'ы по
        # фингерпринту/SNI/IP, которые с облачного чекера не видны.
        c.execute("""CREATE TABLE IF NOT EXISTS probes(
            probe_id TEXT PRIMARY KEY,
            name TEXT,
            token_hash TEXT,
            created_at INTEGER,
            last_seen INTEGER,
            last_geo TEXT)""")
        c.execute("""CREATE TABLE IF NOT EXISTS probe_samples(
            ts INTEGER, probe_id TEXT, sid TEXT,
            ok INTEGER, rtt INTEGER, err TEXT)""")
        c.execute("CREATE INDEX IF NOT EXISTS idx_probe_sid_ts ON probe_samples(sid, ts)")
        c.execute("CREATE INDEX IF NOT EXISTS idx_probe_pid_ts ON probe_samples(probe_id, ts)")
        # Дневные агрегаты по пробникам — для 30-дневной шкалы аптайма у каждой
        # полосы пробника. up/down — счётчики, lat_sum/lat_cnt — для среднего пинга,
        # down_conf — «подтверждённые» простои (2 подряд офлайн = 1 интервал).
        c.execute("""CREATE TABLE IF NOT EXISTS probe_daily(
            day TEXT, probe_id TEXT, sid TEXT,
            up INTEGER DEFAULT 0, down INTEGER DEFAULT 0,
            lat_sum INTEGER DEFAULT 0, lat_cnt INTEGER DEFAULT 0,
            down_conf INTEGER DEFAULT 0,
            PRIMARY KEY(day, probe_id, sid))""")
        c.execute("CREATE INDEX IF NOT EXISTS idx_probe_daily_sid ON probe_daily(sid, day)")
        c.execute("CREATE INDEX IF NOT EXISTS idx_probe_daily_pid ON probe_daily(probe_id, day)")
        # VPN-периоды пробника: если на устройстве в этот момент был включён VPN
        # (geo != EXPECT_COUNTRY), агент шлёт «vpn-репорт» вместо обычного.
        # duration_sec — длительность периода (от предыдущего любого репорта),
        # ограниченная разумным потолком (см. save_probe_vpn). UI рисует
        # серую штриховку на барах дня и серые полосы на графике пинга.
        c.execute("""CREATE TABLE IF NOT EXISTS probe_vpn_samples(
            ts INTEGER, probe_id TEXT,
            duration_sec INTEGER DEFAULT 60,
            PRIMARY KEY(ts, probe_id))""")
        c.execute("CREATE INDEX IF NOT EXISTS idx_probe_vpn_pid_ts ON probe_vpn_samples(probe_id, ts)")


def _get_setting_c(c, k, default=None):
    row = c.execute("SELECT v FROM settings WHERE k=?", (k,)).fetchone()
    return row[0] if row else default


def get_setting(k, default=None):
    with _lock, conn() as c:
        return _get_setting_c(c, k, default)


def set_setting(k, v):
    with _lock, conn() as c:
        c.execute("INSERT INTO settings(k,v) VALUES(?,?) "
                  "ON CONFLICT(k) DO UPDATE SET v=excluded.v",
                  (k, str(v)))


def autoclean_default():
    return "1" if STALE_AFTER_HOURS > 0 else "0"


# ---- Региональные пробники ------------------------------------------------------

# За какое окно (мин) считаем sample пробника «свежим» для статуса на странице.
# Старее этого — UI рисует серую заглушку «нет данных» (но не убирает строку,
# чтобы старый аптайм оставался виден).
PROBE_FRESH_MINUTES = int(os.environ.get("PROBE_FRESH_MINUTES", "10"))
# По умолчанию храним probe-samples под ширину 30-дневной шкалы (+1 на стык),
# чтобы график пинга работал для любого дня из шкалы аптайма.
PROBE_SAMPLE_RETAIN_HOURS = int(os.environ.get(
    "PROBE_SAMPLE_RETAIN_HOURS", str((DAYS + 1) * 24)))
# URL подписки для пробников. Если задан, сервер сам тянет/парсит её и отдаёт
# пробникам разобранный список таргетов через /api/probe/targets — пробник
# больше не лезет в подписку напрямую. Это нужно когда подписка доступна
# только через VPN (агент в РФ не достучится), а сервер в облаке — может.
# Бонусом: секретный URL подписки не светится на устройствах пользователя.
PROBE_SUBSCRIPTION_URL = os.environ.get("PROBE_SUBSCRIPTION_URL", "").strip()
PROBE_TARGETS_TTL_MIN = int(os.environ.get("PROBE_TARGETS_TTL_MIN", "10"))
# Подписки (Remna/Marzban/3x-ui и т.п.) смотрят на User-Agent и отдают разный
# формат в зависимости от клиента, а для незнакомых — заглушку
# «Приложение не поддерживается!». Прикидываемся v2rayN — самый универсальный,
# понимается всеми панелями.
PROBE_SUBSCRIPTION_USER_AGENT = os.environ.get(
    "PROBE_SUBSCRIPTION_USER_AGENT", "v2rayN/6.40")

# In-memory кеш разобранных таргетов: чтобы не дёргать подписку каждый раз.
_targets_cache = {"ts": 0, "data": []}
_targets_lock = threading.Lock()


_PLACEHOLDER_HOSTS = {"0.0.0.0", "127.0.0.1", "::"}


def _parse_vless_line(line):
    """vless://uuid@host:port?security=...&sni=...&fp=...&pbk=...&sid=...
              &flow=...&type=...&alpn=...&path=...&host=...#name
    → словарь с полями, нужными агенту чтобы построить xray-outbound,
    или None если строка невалидна.

    Поля:
      name, host, port, sni, uuid          — базовые
      security                              — "reality" | "tls" | ""
      pbk, sid, fp, flow                    — REALITY/Vision
      type                                  — network: tcp | ws | grpc | h2 | quic
      alpn                                  — CSV (h2, http/1.1)
      path, host_header                     — ws/h2 (host=Host-заголовок)
      header_type                           — для tcp/http obfs
      service_name                          — grpc

    Фильтрует заглушки подписки (0.0.0.0 и т.п.) и пустые UUID."""
    if not line.startswith("vless://"):
        return None
    try:
        p = urlparse(line)
        host = p.hostname
        port = p.port or 443
        if not host or host in _PLACEHOLDER_HOSTS:
            return None
        from urllib.parse import parse_qs, unquote
        q = parse_qs(p.query)
        def get(k, default=""):
            v = q.get(k)
            return (v[0].strip() if v else default)
        sni = q.get("sni") or q.get("peer") or [host]
        sni = sni[0]
        if sni in _PLACEHOLDER_HOSTS:
            sni = host
        name = unquote(p.fragment or "").strip() or host
        uuid_str = (p.username or "").strip()
        if not uuid_str:
            return None
        return {
            "name": name, "host": host, "port": int(port),
            "sni": sni, "uuid": uuid_str,
            "security": get("security").lower(),
            "pbk": get("pbk"),
            "sid": get("sid"),
            "fp": get("fp") or "chrome",
            "flow": get("flow"),
            "type": (get("type") or "tcp").lower(),
            "alpn": get("alpn"),
            "path": unquote(get("path") or ""),
            "host_header": get("host"),
            "header_type": get("headerType").lower(),
            "service_name": unquote(get("serviceName") or ""),
        }
    except Exception:
        return None


def _fetch_subscription_text():
    """Тянет SUBSCRIPTION_URL, при необходимости декодирует base64."""
    req = urllib.request.Request(
        PROBE_SUBSCRIPTION_URL,
        headers={"User-Agent": PROBE_SUBSCRIPTION_USER_AGENT})
    with urllib.request.urlopen(req, timeout=20) as r:
        raw = r.read().decode("utf-8", "ignore").strip()
    # Подписки часто base64. Если в raw нет prefix'ов — декодируем.
    if "vless://" not in raw and "vmess://" not in raw and "trojan://" not in raw:
        try:
            padded = raw + "=" * (-len(raw) % 4)
            decoded = base64.b64decode(padded).decode("utf-8", "ignore")
            if "vless://" in decoded or "vmess://" in decoded:
                raw = decoded
        except Exception:
            pass
    return raw


def get_probe_targets(force=False):
    """С кешированием на PROBE_TARGETS_TTL_MIN минут возвращает список таргетов.
    force=True — игнорируем кеш и ходим на подписку немедленно (для force-refresh
    при обновлении подписки на стороне владельца). При ошибке тянемки возвращаем
    прошлый кеш (если был), чтобы пробники продолжали работать."""
    if not PROBE_SUBSCRIPTION_URL:
        return []
    now = int(time.time())
    with _targets_lock:
        cache_fresh = (now - _targets_cache["ts"] < PROBE_TARGETS_TTL_MIN * 60
                       and _targets_cache["data"])
        if not force and cache_fresh:
            return _targets_cache["data"]
        try:
            raw = _fetch_subscription_text()
            targets = []
            for line in raw.splitlines():
                t = _parse_vless_line(line.strip())
                if t:
                    targets.append(t)
            _targets_cache["ts"] = now
            _targets_cache["data"] = targets
            return targets
        except Exception as e:
            print("probe targets fetch failed:", e, flush=True)
            return _targets_cache["data"]


def _gen_probe_id():
    return "p-" + os.urandom(4).hex()


def _gen_probe_token():
    return os.urandom(24).hex()


def _hash_token(t):
    import hashlib
    return hashlib.sha256(t.encode("utf-8")).hexdigest()


def find_probe_by_token(token):
    """Возвращает (probe_id, name) или None."""
    if not token:
        return None
    th = _hash_token(token)
    with _lock, conn() as c:
        row = c.execute("SELECT probe_id, name FROM probes WHERE token_hash=?",
                        (th,)).fetchone()
    return row if row else None


def create_probe(name, mode="merge"):
    """Регистрирует пробника. Три режима:

    - "merge" (default): если есть пробник с таким именем — переиспользуем
      его probe_id, обновляем token_hash, возвращаем новый токен. История
      (probe_samples/probe_daily) сохраняется — это нужно когда пользователь
      переустанавливает агента после рестарта Mac/Windows и хочет, чтобы
      30-дневный график не пропал.
    - "new": всегда создаём новый probe_id. Старые с тем же именем остаются;
      их данные будут отображаться в той же полосе (мердж по имени в UI).
    - "replace": сносим всех существующих с тем же именем (вместе с их
      probe_samples/probe_daily) и создаём свежего. Используется для полного
      сброса истории пробника.
    """
    name = (name or "").strip() or "probe"
    mode = (mode or "merge").lower()
    if mode not in ("merge", "new", "replace"):
        mode = "merge"
    tok = _gen_probe_token()
    now = int(time.time())
    removed = 0
    reused = False
    with _lock, conn() as c:
        existing = c.execute(
            "SELECT probe_id, created_at FROM probes WHERE name=? "
            "ORDER BY created_at", (name,)).fetchall()
        if mode == "replace" and existing:
            for opid, _ in existing:
                c.execute("DELETE FROM probes WHERE probe_id=?", (opid,))
                c.execute("DELETE FROM probe_samples WHERE probe_id=?", (opid,))
                c.execute("DELETE FROM probe_daily WHERE probe_id=?", (opid,))
            removed = len(existing)
            existing = []
        if mode == "merge" and existing:
            # Переиспользуем самый ранний probe_id, обновляем токен.
            pid = existing[0][0]
            c.execute(
                "UPDATE probes SET token_hash=?, last_seen=? WHERE probe_id=?",
                (_hash_token(tok), now, pid))
            reused = True
            created_at = existing[0][1] or now
        else:
            pid = _gen_probe_id()
            c.execute(
                "INSERT INTO probes(probe_id, name, token_hash, created_at) "
                "VALUES(?,?,?,?)",
                (pid, name, _hash_token(tok), now))
            created_at = now
    return {"probe_id": pid, "name": name, "probe_token": tok,
            "created_at": created_at, "mode": mode,
            "reused": reused, "replaced": removed}


def list_probes():
    t = tz()
    with _lock, conn() as c:
        rows = c.execute(
            "SELECT probe_id, name, created_at, last_seen, last_geo "
            "FROM probes ORDER BY created_at DESC").fetchall()
    out = []
    for pid, name, ca, ls, geo in rows:
        out.append({
            "probe_id": pid, "name": name,
            "createdAt": datetime.fromtimestamp(ca, t).strftime("%Y-%m-%d %H:%M") if ca else "",
            "lastSeen": datetime.fromtimestamp(ls, t).strftime("%Y-%m-%d %H:%M") if ls else None,
            "lastSeenTs": ls or 0,
            "geo": geo or "",
        })
    return out


def delete_probe(pid):
    if not pid:
        return False
    with _lock, conn() as c:
        cur = c.execute("DELETE FROM probes WHERE probe_id=?", (pid,))
        c.execute("DELETE FROM probe_samples WHERE probe_id=?", (pid,))
        c.execute("DELETE FROM probe_daily WHERE probe_id=?", (pid,))
        c.execute("DELETE FROM probe_vpn_samples WHERE probe_id=?", (pid,))
        return cur.rowcount > 0


def wipe_history():
    """Стирает ВСЮ историю опросов: probe_samples/probe_daily/probe_vpn_samples
    + legacy daily/samples. НЕ трогает регистрации пробников (probes) и список
    серверов (current) — агент продолжит слать, серверы из подписки останутся,
    шкала аптайма просто начнёт копиться заново. Нужно когда накопились
    неактуальные данные (например, после отладки) и их надо обнулить."""
    with _lock, conn() as c:
        for tbl in ("probe_samples", "probe_daily", "probe_vpn_samples",
                    "daily", "samples"):
            try:
                c.execute("DELETE FROM %s" % tbl)
            except Exception as e:
                print("wipe_history: %s — %s" % (tbl, e), flush=True)
    return True


def save_probe_vpn(probe_id, geo):
    """Агент задетектил VPN на устройстве (geo != EXPECT_COUNTRY). Пишем
    одну запись в probe_vpn_samples. duration_sec = время от предыдущего
    любого репорта того же пробника (но не больше потолка, чтобы пробуждение
    после долгого простоя не зарисовало весь день VPN-ом).

    Потолок = 2 × PROBE_FRESH_MINUTES × 60. Если разрыв больше — считаем
    как короткий пик (60с). Это покрывает обычный INTERVAL=60s ±дрейф."""
    now = int(time.time())
    cap = max(PROBE_FRESH_MINUTES, 1) * 60 * 2
    with _lock, conn() as c:
        # prev_ts = максимум из probe_samples и probe_vpn_samples этого probe.
        prev = c.execute(
            "SELECT MAX(ts) FROM ("
            " SELECT MAX(ts) AS ts FROM probe_samples WHERE probe_id=?"
            " UNION ALL"
            " SELECT MAX(ts) AS ts FROM probe_vpn_samples WHERE probe_id=?"
            ")", (probe_id, probe_id)).fetchone()
        prev_ts = (prev[0] or 0) if prev else 0
        gap = now - prev_ts if prev_ts else 0
        duration = gap if 0 < gap <= cap else 60
        c.execute(
            "INSERT OR REPLACE INTO probe_vpn_samples(ts, probe_id, duration_sec) "
            "VALUES(?,?,?)",
            (now, probe_id, int(duration)))
        c.execute("UPDATE probes SET last_seen=?, last_geo=? WHERE probe_id=?",
                  (now, (geo or "")[:8], probe_id))
        # Retain — то же окно, что у probe_samples.
        c.execute("DELETE FROM probe_vpn_samples WHERE ts < ?",
                  (now - PROBE_SAMPLE_RETAIN_HOURS * 3600,))
    return 1


def save_probe_report(probe_id, geo, results):
    """Принимает список {name|sid, ok, rtt, err} от пробника, мапит name→canon_sid,
    пишет в probe_samples и обновляет last_seen/last_geo пробника.

    Если пробник прислал несколько результатов с одним `name` (для разных
    sub-серверов одной группы — альтернирующий роутинг), мерджим по логике
    «любой ok → группа ok», rtt = минимум среди ok-членов."""
    now = int(time.time())
    saved = 0
    with _lock, conn() as c:
        # Карта name → canonical sid (минимальный seq среди членов группы).
        name_to_sid = {}
        for sid, name, seq in c.execute("SELECT sid, name, seq FROM current").fetchall():
            cur = name_to_sid.get(name)
            if cur is None or seq < cur[1]:
                name_to_sid[name] = (sid, seq)
        # Сначала разрешаем sid для каждого результата, потом мерджим по sid.
        merged = {}  # sid -> {ok, rtt, err}
        for r in results or []:
            sid = (r.get("sid") or "").strip()
            if not sid:
                nm = (r.get("name") or "").strip()
                pair = name_to_sid.get(nm)
                if not pair:
                    continue
                sid = pair[0]
            ok = bool(r.get("ok"))
            rtt = int(r.get("rtt") or 0)
            err = (r.get("err") or "")[:200] if not ok else ""
            cur = merged.get(sid)
            if cur is None:
                merged[sid] = {"ok": ok, "rtt": rtt, "err": err}
            else:
                if ok and not cur["ok"]:
                    cur["ok"] = True
                    cur["rtt"] = rtt
                    cur["err"] = ""
                elif ok and cur["ok"]:
                    if rtt > 0 and (cur["rtt"] == 0 or rtt < cur["rtt"]):
                        cur["rtt"] = rtt
                # if not ok — оставляем как есть; либо там уже ok, либо тоже не ok
        today = datetime.now(tz()).strftime("%Y-%m-%d")
        for sid, m in merged.items():
            ok = 1 if m["ok"] else 0
            rtt = m["rtt"]
            # down_conf: «подтверждённый» простой — 1, если этот результат офлайн
            # И прошлый sample того же probe×sid тоже офлайн (и был недавно).
            # Так одиночный блип не считается за интервал простоя.
            down_conf = 0
            if not ok:
                prev = c.execute(
                    "SELECT ok, ts FROM probe_samples "
                    "WHERE probe_id=? AND sid=? ORDER BY ts DESC LIMIT 1",
                    (probe_id, sid)).fetchone()
                if prev and prev[0] == 0 and prev[1] is not None:
                    # Если интервал между этим и прошлым sample разумный
                    # (не больше 2× окна свежести) — считаем подтверждённым.
                    if (now - prev[1]) <= PROBE_FRESH_MINUTES * 60 * 2:
                        down_conf = 1
            c.execute(
                "INSERT INTO probe_samples(ts, probe_id, sid, ok, rtt, err) "
                "VALUES(?,?,?,?,?,?)",
                (now, probe_id, sid, ok, rtt, m["err"]))
            lat_sum = rtt if (ok and rtt > 0) else 0
            lat_cnt = 1 if (ok and rtt > 0) else 0
            c.execute(
                "INSERT INTO probe_daily(day, probe_id, sid, up, down, "
                "lat_sum, lat_cnt, down_conf) VALUES(?,?,?,?,?,?,?,?) "
                "ON CONFLICT(day, probe_id, sid) DO UPDATE SET "
                "  up=up+excluded.up, down=down+excluded.down, "
                "  lat_sum=lat_sum+excluded.lat_sum, lat_cnt=lat_cnt+excluded.lat_cnt, "
                "  down_conf=down_conf+excluded.down_conf",
                (today, probe_id, sid, ok, 1 - ok, lat_sum, lat_cnt, down_conf))
            saved += 1
        c.execute("UPDATE probes SET last_seen=?, last_geo=? WHERE probe_id=?",
                  (now, (geo or "")[:8], probe_id))
        c.execute("DELETE FROM probe_samples WHERE ts < ?",
                  (now - PROBE_SAMPLE_RETAIN_HOURS * 3600,))
        # Чистка daily-агрегатов старше окна шкалы (DAYS+1)
        cutoff_day = (datetime.now(tz()) - timedelta(days=DAYS + 1)).strftime("%Y-%m-%d")
        c.execute("DELETE FROM probe_daily WHERE day < ?", (cutoff_day,))
    return saved


def delete_server(sid):
    """Удаляет всю группу записей с тем же `name`, что и у переданного `sid`.

    Один и тот же хост в xray-checker может быть представлен несколькими
    `stableId` (sub-серверы под общим роутингом): у них совпадает `name`, а
    опрашиваются они по очереди. Логически это один хост, поэтому ручное
    удаление должно затрагивать всю группу, иначе остались бы «полу-призраки»."""
    if not sid:
        return 0
    with _lock, conn() as c:
        row = c.execute("SELECT name FROM current WHERE sid=?", (sid,)).fetchone()
        if not row:
            return 0
        name = row[0]
        members = [r[0] for r in c.execute(
            "SELECT sid FROM current WHERE name=?", (name,)).fetchall()]
        for s in members:
            c.execute("DELETE FROM current WHERE sid=?", (s,))
            c.execute("DELETE FROM daily   WHERE sid=?", (s,))
            c.execute("DELETE FROM samples WHERE sid=?", (s,))
        return len(members)


def _sid_for_target(t):
    """Стабильный sid для строки подписки. Хешируем host+port+sni+uuid:
    при смене конфига (новый UUID, sni, порт) sid меняется — что и логично,
    это уже другой сервер. UI группирует разные sid'ы с одним `name` в одну
    строку (как и раньше)."""
    import hashlib
    key = "%s|%d|%s|%s" % (t.get("host") or "", int(t.get("port") or 0),
                           t.get("sni") or "", t.get("uuid") or "")
    return "s" + hashlib.sha256(key.encode("utf-8")).hexdigest()[:12]


def poll_once():
    """Тянем подписку (через тот же кеш get_probe_targets, что используют пробники)
    и синхронизируем `current` — таблицу серверов для UI. xray-checker больше не нужен
    как зависимость: имя берём из фрагмента vless-ссылки, флаг — `detect_country(name)`,
    порядок — позиция строки в подписке."""
    targets = get_probe_targets(force=False)
    if not targets:
        return
    now = int(time.time())
    fresh_sids = set()
    with _lock, conn() as c:
        for seq, t in enumerate(targets):
            sid = _sid_for_target(t)
            fresh_sids.add(sid)
            name = t.get("name") or sid
            # online/latency колонки не используются в UI (статус — из probe-данных),
            # но оставляем ненулевыми для backward-compat существующих БД.
            c.execute("""INSERT INTO current(sid,name,online,latency,ts,seq)
                VALUES(?,?,?,?,?,?)
                ON CONFLICT(sid) DO UPDATE SET
                  name=excluded.name, ts=excluded.ts, seq=excluded.seq""",
                      (sid, name, 1, 0, now, seq))
        # Авточистка устаревших sid'ов: те, что отсутствуют в свежей подписке.
        # Группируем по name; группа жива, если хотя бы один sid пришёл в свежей
        # подписке ИЛИ хотя бы один член моложе stale_cut (защита от лага
        # источника подписки на стороне владельца).
        ac_on = _get_setting_c(c, "autoclean", autoclean_default()) == "1"
        if STALE_AFTER_HOURS > 0 and ac_on:
            stale_cut = now - STALE_AFTER_HOURS * 3600
            rows = c.execute("SELECT name, sid, ts FROM current").fetchall()
            by_name = {}
            for nm, sid, ts in rows:
                by_name.setdefault(nm, []).append((sid, ts))
            stale = []
            for nm, items in by_name.items():
                alive = any(s in fresh_sids or t >= stale_cut for s, t in items)
                if not alive:
                    stale.extend(s for s, _ in items)
            for sid in stale:
                c.execute("DELETE FROM current WHERE sid=?", (sid,))
                c.execute("DELETE FROM daily   WHERE sid=?", (sid,))
                c.execute("DELETE FROM samples WHERE sid=?", (sid,))
            if stale:
                print("auto-cleanup: removed %d sid(s) not in fresh subscription "
                      "(>%dh stale): %s" % (len(stale), STALE_AFTER_HOURS, stale),
                      flush=True)


def poller():
    first = True
    while True:
        try:
            poll_once()
            first = False
        except Exception as e:
            print("poll error:", e, flush=True)
            if first:
                time.sleep(5)
                continue
        time.sleep(POLL_INTERVAL)


def build_summary():
    """В НОВОЙ модели данные о работоспособности берутся ТОЛЬКО от пробников.
    xray-checker остаётся только источником списка серверов (имена, флаги, группы).

    На каждый сервер у нас N полос — по одной на каждого зарегистрированного
    пробника. У каждой своя 30-дневная шкала аптайма (из probe_daily), свой
    текущий пинг и последний sample. Если за последние PROBE_FRESH_MINUTES от
    пробника ничего не пришло — полоса остаётся, но помечается как stale
    («нет данных N мин»), последние сэмплы остаются видны."""
    t = tz()
    now = int(time.time())
    now_local = datetime.now(t)
    day_list = [(now_local - timedelta(days=i)).strftime("%Y-%m-%d")
                for i in range(DAYS - 1, -1, -1)]
    fresh_cutoff = now - PROBE_FRESH_MINUTES * 60

    with _lock, conn() as c:
        servers = c.execute(
            "SELECT sid,name,online,latency,ts,seq FROM current ORDER BY seq").fetchall()
        # Все зарегистрированные пробники — даже без свежих данных.
        probes_meta = c.execute(
            "SELECT probe_id, name, last_seen, last_geo, created_at "
            "FROM probes ORDER BY created_at").fetchall()
        # Дневные агрегаты по пробникам, для построения 30-дневной шкалы.
        daily_rows = c.execute(
            "SELECT probe_id, sid, day, up, down, lat_sum, lat_cnt, down_conf "
            "FROM probe_daily").fetchall()
        # Самый свежий sample на каждую пару (probe_id, sid) — для статуса/пинга.
        latest_rows = c.execute(
            "SELECT probe_id, sid, ts, ok, rtt, err FROM probe_samples "
            "WHERE ts >= ? - ? ",
            (now, PROBE_SAMPLE_RETAIN_HOURS * 3600)).fetchall()
        # VPN-периоды пробников — для серой штриховки на барах дней. Берём
        # только окно шкалы (DAYS).
        vpn_rows = c.execute(
            "SELECT probe_id, ts, duration_sec FROM probe_vpn_samples "
            "WHERE ts >= ?",
            (now - DAYS * 86400,)).fetchall()

    # probe_id -> {name, ...}
    probe_info = {pid: {"name": nm or pid, "lastSeen": ls or 0,
                       "lastGeo": geo or "", "createdAt": ca or 0}
                  for pid, nm, ls, geo, ca in probes_meta}

    # (probe_id, sid) -> {day: (up, down, lat_sum, lat_cnt, down_conf)}
    daily_by_pid_sid = {}
    for pid, sid, day, up, down, ls_, lc_, dc_ in daily_rows:
        daily_by_pid_sid.setdefault((pid, sid), {})[day] = (
            up, down, ls_, lc_, dc_ or 0)

    # probe_id -> {YYYY-MM-DD: total_seconds}
    vpn_by_pid_day = {}
    # probe_id -> самый свежий ts vpn-сэмпла (для определения «сейчас VPN»).
    latest_vpn_ts = {}
    for pid, ts_, dur in vpn_rows:
        d = datetime.fromtimestamp(ts_, t).strftime("%Y-%m-%d")
        vpn_by_pid_day.setdefault(pid, {})
        vpn_by_pid_day[pid][d] = vpn_by_pid_day[pid].get(d, 0) + int(dur or 60)
        if ts_ > latest_vpn_ts.get(pid, 0):
            latest_vpn_ts[pid] = ts_

    # (probe_id, sid) -> {ts, ok, rtt, err}
    latest_by_pid_sid = {}
    for pid, sid, ts_, ok, rtt, err in latest_rows:
        key = (pid, sid)
        cur = latest_by_pid_sid.get(key)
        if cur is None or ts_ > cur["ts"]:
            latest_by_pid_sid[key] = {"ts": ts_, "ok": bool(ok),
                                      "rtt": int(rtt or 0), "err": err or ""}

    # Группируем sid'ы по raw `name` (то же, что и было).
    groups = {}
    for sid, name, online, latency, ts_, seq in servers:
        groups.setdefault(name, []).append((sid, online, latency, ts_, seq))
    sorted_groups = sorted(groups.items(), key=lambda kv: min(m[4] for m in kv[1]))

    out_servers = []
    tot_up = 0
    tot_total = 0
    tot_down_min = 0
    online_count = 0
    lat_vals = []
    poll_interval_min = max(POLL_INTERVAL / 60.0, 0.1)

    for name, members in sorted_groups:
        member_sids = [m[0] for m in members]
        canon_sid = sorted(members, key=lambda x: x[4])[0][0]
        cc = detect_country(name)

        # Группируем зарегистрированных пробников по ИМЕНИ: несколько probe_id
        # с одним name считаются одним физическим устройством (например,
        # повторная установка install-macos.sh создала дубль). Их данные
        # объединяются в одну полосу.
        probes_by_name = {}
        for pid, pinfo in probe_info.items():
            probes_by_name.setdefault(pinfo["name"], []).append(pid)

        regional = []
        for probe_name in sorted(probes_by_name):
            pids = probes_by_name[probe_name]
            # Дневные агрегаты по всем sid группы и всем probe_id с этим именем.
            days = []
            s_up = 0
            s_total = 0
            s_down_min = 0
            s_vpn_min = 0
            for d in day_list:
                sum_up = sum_total = sum_dc = 0
                n_records = 0
                for sid in member_sids:
                    for pid in pids:
                        rec = daily_by_pid_sid.get((pid, sid), {}).get(d)
                        if rec:
                            n_records += 1
                            sum_up += rec[0]
                            sum_total += rec[0] + rec[1]
                            sum_dc += rec[4]
                # VPN-минуты дня — суммируем по всем pid этого имени.
                vpn_sec_d = sum(vpn_by_pid_day.get(pid, {}).get(d, 0)
                                for pid in pids)
                vpn_min_d = round(vpn_sec_d / 60.0)
                if vpn_min_d > 0:
                    s_vpn_min += vpn_min_d
                if sum_total > 0:
                    pct = round(sum_up / sum_total * 100, 2)
                    down_min_d = round(sum_dc / max(n_records, 1) * poll_interval_min)
                    s_up += sum_up
                    s_total += sum_total
                    s_down_min += down_min_d
                    has_data = True
                else:
                    pct = None
                    down_min_d = 0
                    has_data = vpn_min_d > 0
                y, m, dd = d.split("-")
                label = dd + " " + RU_MONTHS[int(m)]
                days.append({"date": d, "label": label, "uptime": pct,
                             "downMin": down_min_d, "hasData": has_data,
                             "vpnMin": vpn_min_d})
            # Самый свежий sample по группе и всем pid этого имени.
            latest = None
            for sid in member_sids:
                for pid in pids:
                    cand = latest_by_pid_sid.get((pid, sid))
                    if cand and (latest is None or cand["ts"] > latest["ts"]):
                        latest = cand
            # Самый свежий VPN-сэмпл среди всех pid этого имени.
            vpn_ts = max((latest_vpn_ts.get(pid, 0) for pid in pids), default=0)
            sample_ts = latest["ts"] if latest else 0
            # «Свежесть» считаем по ЛЮБОЙ активности — обычный sample ИЛИ vpn-репорт.
            # Устройство в VPN шлёт только vpn-репорты (без results), но оно активно.
            last_activity = max(sample_ts, vpn_ts)
            fresh = bool(last_activity >= fresh_cutoff)
            # «Сейчас VPN» — свежая активность есть, и последней была vpn-отметка.
            vpn_active = bool(fresh and vpn_ts > sample_ts)
            if s_total == 0 and latest is None and s_vpn_min == 0:
                continue
            band = {
                # Канонический probe_id — первый по дате создания. Используется
                # как fallback для legacy-фронтов; новый UI идентифицирует
                # полосу по `probe` (имени).
                "probe_id": pids[0],
                "probe": probe_name,
                "probeIds": pids,
                "fresh": fresh,
                "vpnActive": vpn_active,
                "lastSeenTs": max(sample_ts, vpn_ts),
                "online": (latest["ok"] if (latest and fresh and not vpn_active) else False),
                "latencyMs": (latest["rtt"] if (latest and fresh and not vpn_active and latest["ok"]) else 0),
                "err": (latest["err"] if (latest and not vpn_active and not latest["ok"]) else ""),
                "uptime30": (round(s_up / s_total * 100, 2) if s_total else None),
                "downMin30": s_down_min,
                "vpnMin30": s_vpn_min,
                "days": days,
            }
            regional.append(band)
            # Тоталы: одна группа = одно «онлайн», если хотя бы один свежий пробник видит её.
            # Считаем только один раз на группу (через флаг ниже).

        # Текущий статус группы для тоталов и для верхнего кружка-точки.
        any_fresh_online = any(b["fresh"] and b["online"] for b in regional)
        any_fresh = any(b["fresh"] for b in regional)
        # Текущий пинг (для лимита на странице) — берём минимальный среди свежих ok.
        cur_rtts = [b["latencyMs"] for b in regional
                    if b["fresh"] and b["online"] and b["latencyMs"] > 0]
        cur_latency = min(cur_rtts) if cur_rtts else 0
        # 30-дневный аптайм группы = усреднение по каждому пробнику * его весу
        # (по простому: средний uptime30 у пробников у которых он не None).
        upts = [b["uptime30"] for b in regional if b["uptime30"] is not None]
        up30 = round(sum(upts) / len(upts), 2) if upts else None

        if any_fresh_online:
            online_count += 1
            if cur_latency > 0:
                lat_vals.append(cur_latency)

        sum_dm = sum(b["downMin30"] for b in regional)
        sum_up_all = sum(b["uptime30"] for b in regional if b["uptime30"] is not None)
        out_servers.append({
            "sid": canon_sid,
            "name": display_name(name, cc),
            "cc": cc,
            "online": bool(any_fresh_online),
            "anyFresh": bool(any_fresh),
            "latencyMs": cur_latency,
            "uptime30": up30,
            "downMin30": sum_dm,
            "members": len(members),
            "regional": regional,
            "noData": len(regional) == 0,
        })
        tot_up += sum_up_all
        tot_total += len(upts) * 100  # каждое uptime30 — это «100%-сегмент»
        tot_down_min += sum_dm

    avg_lat = round(sum(lat_vals) / len(lat_vals)) if lat_vals else 0
    totals = {
        "online": online_count,
        "total": len(out_servers),
        "uptime30": round(tot_up / tot_total * 100, 2) if tot_total else None,
        "avgLatency": avg_lat,
        "downMin30": tot_down_min,
        "probes": len(probe_info),
    }
    # «Последняя проверка» = максимум lastSeen среди всех пробников.
    last_ts = max((p["lastSeen"] for p in probe_info.values()), default=0)
    last_check = (datetime.fromtimestamp(last_ts, t).strftime("%Y-%m-%d %H:%M")
                  if last_ts else None)
    return {
        "title": TITLE, "subtitle": SUBTITLE, "days": DAYS,
        "pollInterval": POLL_INTERVAL,
        "freshMinutes": PROBE_FRESH_MINUTES,
        "generatedAt": now_local.strftime("%Y-%m-%d %H:%M"),
        "lastCheck": last_check,
        "servers": out_servers, "totals": totals,
        "adminEnabled": bool(ADMIN_TOKEN),
        "probeMode": True,
    }


def _day_payload(sid, ds, is_today, probe_id=None, probe_name=None):
    """Сэмплы за конкретный день. ТОЛЬКО от пробников.
    Если задан probe_name — резолвим его в список probe_id с этим именем и
    мерджим сэмплы всех (несколько устройств с одним именем = одно физическое).
    Если задан probe_id — используем только его. Без обоих — пустой результат."""
    end = ds + 86400
    upper = int(time.time()) if is_today else end
    samples = []
    pids = []
    vpn_rows = []
    with _lock, conn() as c:
        if probe_name and not probe_id:
            pids = [r[0] for r in c.execute(
                "SELECT probe_id FROM probes WHERE name=?", (probe_name,)).fetchall()]
        elif probe_id:
            pids = [probe_id]
        rows = []
        if pids:
            row = c.execute("SELECT name FROM current WHERE sid=?", (sid,)).fetchone()
            if row:
                members = [r[0] for r in c.execute(
                    "SELECT sid FROM current WHERE name=?", (row[0],)).fetchall()]
            else:
                members = [sid]
            sid_ph = ",".join("?" * len(members))
            pid_ph = ",".join("?" * len(pids))
            rows = c.execute(
                f"SELECT ts, ok, rtt FROM probe_samples "
                f"WHERE probe_id IN ({pid_ph}) AND sid IN ({sid_ph}) "
                "AND ts>=? AND ts<? ORDER BY ts",
                tuple(pids) + tuple(members) + (ds, end)).fetchall()
            # VPN-интервалы: per-pid выборка, потом склейка близких в один.
            vpn_rows = c.execute(
                f"SELECT ts, duration_sec FROM probe_vpn_samples "
                f"WHERE probe_id IN ({pid_ph}) AND ts>=? AND ts<? "
                "ORDER BY ts",
                tuple(pids) + (ds, end)).fetchall()
        # Роллап по ts.
        buckets = {}
        for ts, ok, rtt in rows:
            b = buckets.get(ts)
            if b is None:
                buckets[ts] = [int(ok), int(rtt) if ok else 0]
            else:
                if ok:
                    if not b[0]:
                        b[0] = 1
                        b[1] = int(rtt) if rtt > 0 else 0
                    elif rtt > 0 and (b[1] == 0 or rtt < b[1]):
                        b[1] = int(rtt)
        samples = [{"ts": ts, "online": bool(buckets[ts][0]),
                    "latency": buckets[ts][1]}
                   for ts in sorted(buckets)]
    pings = [s["latency"] for s in samples if s["online"] and s["latency"] > 0]
    stats = {
        "checks": len(samples),
        "errors": sum(1 for s in samples if not s["online"]),
        "pmin": min(pings) if pings else 0,
        "pavg": round(sum(pings) / len(pings)) if pings else 0,
        "pmax": max(pings) if pings else 0,
    }
    # Склейка VPN-семплов в интервалы: каждый sample покрывает [ts-duration, ts].
    # Если конец прошлого и начало следующего отстоят меньше чем на 2*INTERVAL —
    # объединяем в один отрезок (зазоры в пределах одного-двух циклов
    # игнорируем, чтобы не плодить «гребёнку»).
    vpn_intervals = []
    glue = max(PROBE_FRESH_MINUTES, 1) * 60 * 2
    for ts, dur in vpn_rows:
        a = max(ds, ts - int(dur or 60))
        b = min(end, ts)
        if not vpn_intervals or a - vpn_intervals[-1]["end"] > glue:
            vpn_intervals.append({"start": a, "end": b})
        else:
            vpn_intervals[-1]["end"] = max(vpn_intervals[-1]["end"], b)
    vpn_seconds_total = sum(iv["end"] - iv["start"] for iv in vpn_intervals)
    dt = datetime.fromtimestamp(ds, tz())
    label = "Сегодня" if is_today else (dt.strftime("%d ") + RU_MONTHS[dt.month])
    return {"dayStart": ds, "now": upper, "isToday": is_today, "dayLabel": label,
            "pollInterval": PROBE_FRESH_MINUTES * 60,
            "samples": samples, "stats": stats, "probeId": probe_id or "",
            "vpnIntervals": vpn_intervals,
            "vpnMinutes": round(vpn_seconds_total / 60)}


def _midnight_ts(dt_local):
    m = dt_local.replace(hour=0, minute=0, second=0, microsecond=0)
    return int(m.timestamp())


def build_today(sid, probe_id=None, probe_name=None):
    return _day_payload(sid, _midnight_ts(datetime.now(tz())), True,
                        probe_id, probe_name)


def build_day(sid, date_str, probe_id=None, probe_name=None):
    t = tz()
    try:
        d = datetime.strptime(date_str, "%Y-%m-%d")
    except Exception:
        return build_today(sid, probe_id, probe_name)
    ds = int(datetime(d.year, d.month, d.day, tzinfo=t).timestamp())
    today0 = _midnight_ts(datetime.now(t))
    return _day_payload(sid, ds, ds == today0, probe_id, probe_name)


INDEX_HTML = r"""<!DOCTYPE html>
<html lang="ru">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>__TITLE__</title>
__FAVICON__
<style>
@font-face{font-family:'Inter';font-style:normal;font-weight:400;font-display:swap;src:url('fonts/inter-cyrillic-400-normal.woff2') format('woff2');unicode-range:U+0301,U+0400-045F,U+0490-0491,U+04B0-04B1,U+2116;}
@font-face{font-family:'Inter';font-style:normal;font-weight:500;font-display:swap;src:url('fonts/inter-cyrillic-500-normal.woff2') format('woff2');unicode-range:U+0301,U+0400-045F,U+0490-0491,U+04B0-04B1,U+2116;}
@font-face{font-family:'Inter';font-style:normal;font-weight:600;font-display:swap;src:url('fonts/inter-cyrillic-600-normal.woff2') format('woff2');unicode-range:U+0301,U+0400-045F,U+0490-0491,U+04B0-04B1,U+2116;}
@font-face{font-family:'Inter';font-style:normal;font-weight:400;font-display:swap;src:url('fonts/inter-latin-400-normal.woff2') format('woff2');unicode-range:U+0000-00FF,U+0131,U+0152-0153,U+02BB-02BC,U+02C6,U+02DA,U+02DC,U+2000-206F,U+2074,U+20AC,U+2122,U+2191,U+2193,U+2212,U+2215,U+FEFF,U+FFFD;}
@font-face{font-family:'Inter';font-style:normal;font-weight:500;font-display:swap;src:url('fonts/inter-latin-500-normal.woff2') format('woff2');unicode-range:U+0000-00FF,U+0131,U+0152-0153,U+02BB-02BC,U+02C6,U+02DA,U+02DC,U+2000-206F,U+2074,U+20AC,U+2122,U+2191,U+2193,U+2212,U+2215,U+FEFF,U+FFFD;}
@font-face{font-family:'Inter';font-style:normal;font-weight:600;font-display:swap;src:url('fonts/inter-latin-600-normal.woff2') format('woff2');unicode-range:U+0000-00FF,U+0131,U+0152-0153,U+02BB-02BC,U+02C6,U+02DA,U+02DC,U+2000-206F,U+2074,U+20AC,U+2122,U+2191,U+2193,U+2212,U+2215,U+FEFF,U+FFFD;}
:root,html[data-theme="light"]{
  --bg:#fbfcfe; --card:#ffffff; --soft:#f1f5fb; --line:#e4e9f0; --hover:#f3f7fd;
  --tx:#13171d; --tx2:#46505f; --tx3:#69727f;
  --ok:#13a06f; --warn:#e0951a; --orange:#dc5b25; --bad:#e23f3d; --info:#2f6bff;
  --shadow:0 1px 2px rgba(18,28,45,.06),0 1px 3px rgba(18,28,45,.05);
}
@media (prefers-color-scheme: dark){
  :root{--bg:#16181d; --card:#1f232a; --soft:#23272f; --line:#333a45; --hover:#262b33;
    --tx:#f1f4f9; --tx2:#b2bbc8; --tx3:#8d97a5;
    --ok:#26c089; --warn:#f2b13e; --orange:#ec7242; --bad:#f25c5a; --info:#5b8cff; --shadow:none;}
}
html[data-theme="dark"]{--bg:#16181d; --card:#1f232a; --soft:#23272f; --line:#333a45; --hover:#262b33;
  --tx:#f1f4f9; --tx2:#b2bbc8; --tx3:#8d97a5;
  --ok:#26c089; --warn:#f2b13e; --orange:#ec7242; --bad:#f25c5a; --info:#5b8cff; --shadow:none;}
/* Claude — тёплая кремовая палитра с фирменным copper-оранжевым акцентом */
html[data-theme="claude"]{
  --bg:#F0EEE6; --card:#FAF9F5; --soft:#E8E5DA; --line:#D9D5C6; --hover:#ECE9DD;
  --tx:#1F1E1D; --tx2:#4B4337; --tx3:#7A7363;
  --ok:#5F8A56; --warn:#C48923; --orange:#C76A47; --bad:#B44A3A; --info:#D97757;
  --shadow:0 1px 2px rgba(50,30,15,.05),0 1px 3px rgba(50,30,15,.04);
}
html[data-theme="claude"] body{background:radial-gradient(1200px 600px at 50% -200px,#F5F2E8 0%,#F0EEE6 60%) no-repeat fixed,var(--bg);}
html[data-theme="claude"] .logo{background:rgba(217,119,87,.13);}
html[data-theme="claude"] .pill.ok{background:rgba(95,138,86,.14);color:var(--ok);}
html[data-theme="claude"] .pill.bad{background:rgba(180,74,58,.14);color:var(--bad);}
html[data-theme="claude"] #lock.lockon{background:rgba(217,119,87,.15);color:var(--info);border-color:rgba(217,119,87,.45);}
html[data-theme="claude"] .delbtn:hover{background:rgba(180,74,58,.12);color:var(--bad);border-color:var(--bad);}
html[data-theme="claude"] .item{border-color:#DDD8C8;}
html[data-theme="claude"] .item:hover{border-color:#C4BDA8;}
@keyframes pulseClaude{0%{box-shadow:0 0 0 0 rgba(95,138,86,.45);}70%{box-shadow:0 0 0 6px rgba(95,138,86,0);}100%{box-shadow:0 0 0 0 rgba(95,138,86,0);}}
html[data-theme="claude"] .pill.ok .dot{animation:pulseClaude 2.4s ease-out infinite;}
/* Claude Code — тёплая тёмная палитра (CLI-вайб) с copper-оранжевым акцентом */
html[data-theme="claude-dark"]{
  --bg:#1A1815; --card:#262320; --soft:#2D2A26; --line:#3F3933; --hover:#302C28;
  --tx:#ECE7D9; --tx2:#B8AE9A; --tx3:#857D6C;
  --ok:#87A571; --warn:#D9A05B; --orange:#D97757; --bad:#D77565; --info:#D97757;
  --shadow:none;
}
html[data-theme="claude-dark"] body{background:radial-gradient(1400px 700px at 50% -200px,#23201C 0%,#1A1815 60%) no-repeat fixed,var(--bg);}
html[data-theme="claude-dark"] .logo{background:rgba(217,119,87,.16);}
html[data-theme="claude-dark"] .pill.ok{background:rgba(135,165,113,.16);color:var(--ok);}
html[data-theme="claude-dark"] .pill.bad{background:rgba(215,117,101,.18);color:var(--bad);}
html[data-theme="claude-dark"] #lock.lockon{background:rgba(217,119,87,.18);color:var(--info);border-color:rgba(217,119,87,.5);}
html[data-theme="claude-dark"] .delbtn:hover{background:rgba(215,117,101,.18);color:var(--bad);border-color:var(--bad);}
html[data-theme="claude-dark"] .actoggle::before{background:#ECE7D9;}
@keyframes pulseClaudeDark{0%{box-shadow:0 0 0 0 rgba(135,165,113,.45);}70%{box-shadow:0 0 0 6px rgba(135,165,113,0);}100%{box-shadow:0 0 0 0 rgba(135,165,113,0);}}
html[data-theme="claude-dark"] .pill.ok .dot{animation:pulseClaudeDark 2.4s ease-out infinite;}
*{box-sizing:border-box}
html{overflow-y:scroll;scrollbar-gutter:stable;}
body{margin:0;background:var(--bg);color:var(--tx);
  font-family:'Inter',-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif;
  -webkit-font-smoothing:antialiased;text-rendering:optimizeLegibility;}
@keyframes fadeUp{from{opacity:0;transform:translateY(7px);}to{opacity:1;transform:none;}}
@keyframes pulse{0%{box-shadow:0 0 0 0 rgba(22,176,122,.45);}70%{box-shadow:0 0 0 6px rgba(22,176,122,0);}100%{box-shadow:0 0 0 0 rgba(22,176,122,0);}}
@media (prefers-reduced-motion: reduce){*{animation:none !important;transition:none !important;}}
.wrap{max-width:940px;margin:0 auto;padding:30px 18px 52px;}
.top{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;margin-bottom:20px;}
.brand{display:flex;align-items:center;gap:12px;}
.logo{width:38px;height:38px;border-radius:10px;background:rgba(47,107,255,.12);color:var(--info);
  display:flex;align-items:center;justify-content:center;}
.brand h1{font-size:20px;font-weight:600;margin:0;line-height:1.2;}
.brand p{font-size:13px;color:var(--tx2);margin:3px 0 0;}
.pill{display:flex;align-items:center;gap:8px;padding:8px 15px;border-radius:999px;
  font-size:13.5px;font-weight:500;}
.pill .dot{width:8px;height:8px;border-radius:50%;display:inline-block;}
.pill.ok{background:rgba(22,176,122,.13);color:var(--ok);}
.pill.bad{background:rgba(232,80,78,.13);color:var(--bad);}
.pill.ok .dot{animation:pulse 2.4s ease-out infinite;}
.topr{display:flex;align-items:center;gap:10px;}
.tbtn{display:flex;align-items:center;justify-content:center;width:36px;height:36px;flex:none;
  border-radius:10px;border:1px solid var(--line);background:var(--card);color:var(--tx2);cursor:pointer;
  transition:background .15s,border-color .15s,color .15s;}
.tbtn:hover{background:var(--hover);color:var(--tx);border-color:var(--tx3);}
.stats{display:grid;grid-template-columns:repeat(3,1fr);gap:14px;margin-bottom:18px;}
.stat{background:var(--card);border:1px solid var(--line);border-radius:14px;padding:14px 18px;box-shadow:var(--shadow);}
.stat .l{font-size:13px;color:var(--tx2);}
.stat .v{font-size:25px;font-weight:600;margin-top:3px;letter-spacing:-.3px;}
.item{background:var(--card);border:1px solid var(--line);border-radius:14px;margin-bottom:9px;
  box-shadow:var(--shadow);overflow:hidden;animation:fadeUp .34s ease both;
  transition:border-color .15s, box-shadow .15s;}
.item:hover{border-color:rgba(127,140,158,.42);box-shadow:0 2px 10px rgba(18,28,45,.07);}
.row{display:flex;align-items:center;gap:14px;padding:12px 15px;cursor:pointer;transition:background .14s;}
.row:hover{background:var(--hover);}
.label{width:214px;flex:none;display:flex;align-items:center;gap:9px;min-width:0;}
.flag{width:23px;height:16px;border-radius:3px;object-fit:cover;border:1px solid rgba(0,0,0,.12);flex:none;
  background:var(--soft);}
.nm{min-width:0;}
.name{display:flex;align-items:center;gap:8px;font-size:15px;font-weight:500;}
.name .sdot{width:9px;height:9px;border-radius:50%;flex:none;transition:background-color .3s;}
.name span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;}
.bars{flex:1;display:block;height:26px;min-width:0;}
.bars rect{transition:fill .3s, opacity .12s;cursor:pointer;}
.bars rect:hover{opacity:.55;}
/* margin-left:auto прижимает stat2 (и идущие за ним chev/delbtn) к правому
   краю строки. В probe-only режиме 30-дневные бары переехали в bands, и без
   этого спейсера статистика и кнопки «закрыть/удалить» паковались влево. */
.stat2{width:124px;flex:none;text-align:right;margin-left:auto;}
.stat2 .p{font-size:15px;font-weight:600;}
.stat2 .s{font-size:12px;color:var(--tx3);}
.chev{color:var(--tx3);margin-left:2px;flex:none;display:flex;align-items:center;justify-content:center;
  width:26px;height:26px;border-radius:50%;transition:transform .22s ease, background-color .15s, color .15s;}
.row:hover .chev{background:rgba(127,140,158,.14);color:var(--tx2);}
.item.open .chev{transform:rotate(180deg);}
.panel{max-height:0;overflow:hidden;opacity:0;background:var(--soft);border-top:1px solid transparent;
  padding:0 20px;transition:max-height .3s ease, opacity .24s ease, padding .28s ease, border-color .28s ease;}
.item.open .panel{opacity:1;padding:20px 20px 22px;border-top-color:var(--line);}
.phead{font-size:13px;color:var(--tx2);margin-bottom:16px;line-height:1.5;}
.tstats{display:flex;flex-wrap:wrap;gap:16px 34px;margin-bottom:4px;}
.tstats div{display:flex;flex-direction:column;gap:3px;}
.tstats span{font-size:12.5px;color:var(--tx2);}
.tstats b{font-size:18px;font-weight:600;letter-spacing:-.2px;}
.tcaption{font-size:12.5px;color:var(--tx3);margin:20px 0 9px;}
.tchartwrap{width:100%;position:relative;}
.tscroll{overflow-x:auto;overflow-y:hidden;scrollbar-width:thin;scrollbar-color:var(--line) transparent;}
.tscroll::-webkit-scrollbar{height:9px;}
.tscroll::-webkit-scrollbar-track{background:transparent;}
.tscroll::-webkit-scrollbar-thumb{background:var(--line);border-radius:9px;border:2px solid transparent;background-clip:content-box;}
.tscroll:hover::-webkit-scrollbar-thumb{background:var(--tx3);background-clip:content-box;}
.tcanvas{position:relative;padding-bottom:10px;}
.tchart{display:block;width:100%;height:150px;}
.tyaxis{position:absolute;right:6px;z-index:2;transform:translateY(-50%);font-size:12px;color:var(--tx3);
  background:var(--soft);padding:0 4px;pointer-events:none;border-radius:3px;}
.taxis{display:flex;justify-content:space-between;font-size:12px;color:var(--tx3);margin-top:9px;}
.empty{color:var(--tx2);font-size:13px;padding:6px 0;}
.legend{display:flex;align-items:center;gap:16px;flex-wrap:wrap;margin-top:16px;font-size:13px;color:var(--tx2);}
.legend i{width:12px;height:12px;border-radius:3px;display:inline-block;margin-right:6px;vertical-align:-1px;}
.legend .right{margin-left:auto;}
.foot{margin-top:22px;font-size:13px;color:var(--tx3);text-align:center;}
#tip{position:fixed;pointer-events:none;opacity:0;transition:opacity .08s;z-index:60;
  background:var(--card);border:1px solid var(--line);border-radius:10px;padding:9px 11px;
  font-size:13px;min-width:140px;box-shadow:0 8px 28px rgba(18,28,45,.16);}
#tip .d{font-weight:600;margin-bottom:4px;}
#tip .k{color:var(--tx2);line-height:1.5;}
.skel{color:var(--tx2);font-size:14px;padding:30px 0;text-align:center;}
.delbtn{display:flex;align-items:center;justify-content:center;width:30px;height:30px;flex:none;
  border-radius:8px;border:1px solid var(--line);background:transparent;color:var(--tx3);cursor:pointer;
  margin-left:2px;transition:background .15s,color .15s,border-color .15s;}
.delbtn:hover{background:rgba(232,80,80,.12);color:var(--bad);border-color:var(--bad);}
#lock.lockon{background:rgba(47,107,255,.13);color:var(--info);border-color:var(--info);}
.adminbar{display:flex;flex-direction:column;
  background:var(--card);border:1px solid var(--line);border-radius:14px;
  padding:4px 16px;margin-bottom:18px;box-shadow:var(--shadow);
  animation:fadeUp .34s ease both;}
.adminrow{display:flex;align-items:center;gap:16px;padding:12px 0;}
.adminrow + .adminrow{border-top:1px solid var(--line);}
.adminlabel{flex:1;font-size:14px;color:var(--tx);min-width:0;}
.adminlabel small{display:block;font-size:12.5px;color:var(--tx3);margin-top:2px;font-weight:400;}
.adminlabel.aclocked small{color:var(--bad);}
.actoggle{position:relative;width:48px;height:28px;border-radius:999px;flex:none;
  background:var(--line);border:0;cursor:pointer;padding:0;
  transition:background .18s ease;}
.actoggle::before{content:"";position:absolute;top:3px;left:3px;width:22px;height:22px;
  border-radius:50%;background:#fff;box-shadow:0 1px 2px rgba(18,28,45,.18);
  transition:transform .22s ease;}
.actoggle.actogon{background:var(--ok);}
.actoggle.actogon::before{transform:translateX(20px);}
.actoggle:disabled{opacity:.45;cursor:not-allowed;}
.bands{display:flex;flex-direction:column;gap:4px;padding:4px 15px 12px;
  border-top:1px dashed var(--line);margin:2px 0 0;}
/* flex-wrap: чтобы строка ошибки (.bandErr, flex-basis:100%) переносилась
   на отдельную строку под основным рядом и показывалась целиком. */
.band{display:flex;flex-wrap:wrap;align-items:center;gap:6px 12px;cursor:pointer;
  padding:5px 8px;border-radius:8px;transition:background .14s;}
.band:hover{background:var(--hover);}
.bandName{display:flex;align-items:center;gap:7px;width:140px;flex:none;min-width:0;
  font-size:12.5px;color:var(--tx2);}
.bandName .bandPName{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;}
.bandDot{width:7px;height:7px;border-radius:50%;flex:none;}
.bandDotOk{background:var(--ok);}                 /* активен, сервер онлайн — зелёный */
.bandDotBad{background:var(--bad);}               /* активен, сервер офлайн — красный */
.bandDotVpn{background:#969aa6;}                  /* на устройстве включён VPN — серый */
.bandDotStale{background:#3d7bf0;}                /* устройство неактивно (не шлёт) — синий */
.bandBars{flex:1;height:18px;min-width:0;display:block;}
.bandBars rect{transition:fill .3s, opacity .12s;}
.bandBars rect:hover{opacity:.55;}
/* Оверлей VPN на бары дней — серая полупрозрачная заливка, чтобы было
   видно, что в этот день у пробника часть времени работал VPN (за этот
   период мы не пытались тестить — пробы через туннель бессмысленны). */
.bandBars rect.vpn{fill:rgba(160,160,160,.45);pointer-events:none;}
.bandStat{width:110px;flex:none;text-align:right;}
.bandPct{font-size:12.5px;font-weight:500;}
.bandSub{font-size:11px;color:var(--tx3);}
/* Полноширинная строка с полным текстом ошибки пробника (когда офлайн).
   flex-basis:100% переносит её под основной ряд band'а; видно целиком. */
.bandErr{flex-basis:100%;width:100%;font-size:11px;line-height:1.35;
  color:var(--bad);padding:1px 2px 2px 24px;word-break:break-word;}
.emptyBands{padding:10px 15px 14px;font-size:12.5px;color:var(--tx3);font-style:italic;
  border-top:1px dashed var(--line);}
@media (max-width:560px){
  .wrap{padding:22px 14px 40px;}
  .brand h1{font-size:18px;} .brand p{font-size:12px;}
  .stats{grid-template-columns:1fr 1fr;gap:10px;}
  .stat{padding:12px 14px;border-radius:12px;} .stat .v{font-size:22px;}
  .row{flex-wrap:wrap;gap:9px 12px;padding:12px 14px;}
  .label{width:auto;flex:1 1 auto;min-width:0;}
  .stat2{width:auto;text-align:right;}
  .chev{order:4;}
  .delbtn{order:3;}
  .bars{order:5;flex-basis:100%;height:26px;}
  .legend{gap:9px 14px;margin-top:14px;} .legend .right{display:none;}
  .tstats{gap:12px 22px;} .panel{padding:0 14px;}
  .item.open .panel{padding:16px 14px 18px;}
}
</style>
<script>try{var _t=localStorage.getItem("sp-theme");if(_t)document.documentElement.setAttribute("data-theme",_t);}catch(e){}</script>
</head>
<body>
<div class="wrap">
  <div class="top">
    <div class="brand">
      <div class="logo">__LOGO__</div>
      <div><h1 id="title">__TITLE__</h1><p id="subtitle">__SUBTITLE__</p></div>
    </div>
    <div class="topr">
      <div id="overall" class="pill ok"><span class="dot"></span><span>Загрузка…</span></div>
      <button id="lock" class="tbtn" hidden aria-label="Админ-режим" title="Админ-режим"></button>
      <button id="theme-btn" class="tbtn" aria-label="Сменить тему" title="Сменить тему"></button>
    </div>
  </div>
  <div class="stats">
    <div class="stat"><div class="l">Серверов онлайн</div><div class="v" id="s-online">—</div></div>
    <div class="stat"><div class="l">Аптайм за __DAYS__ дн</div><div class="v" id="s-uptime">—</div></div>
    <div class="stat"><div class="l">Средний пинг</div><div class="v" id="s-ping">—</div></div>
  </div>
  <div id="adminbar" class="adminbar" hidden>
    <div class="adminrow">
      <div id="adminlabel" class="adminlabel">
        Авто-удаление устаревших записей
        <small id="ac-sub">через <b id="ac-hours">—</b> ч после исчезновения из подписки</small>
      </div>
      <button id="ac-toggle" class="actoggle" type="button" aria-label="Авто-удаление"></button>
    </div>
    <div class="adminrow">
      <div class="adminlabel">
        Очистить историю опросов
        <small>сбросить все накопленные данные пробников (графики, аптайм). Регистрации устройств и список серверов сохранятся.</small>
      </div>
      <button id="wipe-btn" class="delbtn" type="button" aria-label="Очистить историю" title="Очистить историю опросов" style="width:auto;padding:0 14px;gap:6px;">Очистить</button>
    </div>
  </div>
  <div id="list"><div class="skel">Загрузка данных…</div></div>
  <div class="legend">
    <span><i style="background:#16b07a"></i>норма</span>
    <span><i style="background:#f0a82a"></i>до 30 мин</span>
    <span><i style="background:#e3692f"></i>30 мин – 2 ч</span>
    <span><i style="background:#e8504e"></i>от 2 ч</span>
    <span><i style="background:#cfd6df"></i>нет данных</span>
    <span class="right">← __DAYS__ дней назад · сегодня →</span>
  </div>
  <div class="foot" id="foot"></div>
</div>
<div id="tip"></div>
<script>
var GREY="#cfd6df";
var SVGNS="http://www.w3.org/2000/svg";
function colorFor(d){
  if(!d.hasData) return GREY;
  if(d.downMin<=0) return "#16b07a";
  if(d.downMin<=30) return "#f0a82a";
  if(d.downMin<120) return "#e3692f";
  return "#e8504e";
}
function fmtDur(m){
  if(m<=0) return "0 мин";
  if(m>=1440){var dd=Math.floor(m/1440),h=Math.round((m%1440)/60);return dd+" дн"+(h?" "+h+" ч":"");}
  if(m>=60){var h=Math.floor(m/60),mm=m%60;return h+" ч"+(mm?" "+mm+" мин":"");}
  return m+" мин";
}
function fmtTime(ts,ds){
  var off=ts-ds,h=Math.floor(off/3600),m=Math.floor((off%3600)/60);
  return ("0"+h).slice(-2)+":"+("0"+m).slice(-2);
}
function escapeHtml(s){var d=document.createElement("div");d.textContent=s;return d.innerHTML;}
var tip=document.getElementById("tip");
function evXY(e){var t=e.touches&&e.touches[0];return t||e;}
function moveTip(e){
  e=evXY(e);
  var w=tip.offsetWidth,x=e.clientX+14,y=e.clientY-10;
  if(x+w>window.innerWidth-8)x=e.clientX-w-14;
  tip.style.left=x+"px";tip.style.top=Math.max(8,y)+"px";
}
function hideTip(){tip.style.opacity="0";}
function showTipDay(e,d){
  var u=d.uptime;
  var uc=(u===null)?"var(--tx2)":(u>=99.99?"var(--ok)":(u>=95?"var(--warn)":"var(--bad)"));
  var ut=(u===null)?"нет данных":u.toFixed(2)+"%";
  var down=!d.hasData?''
    :d.downMin>0?'<div class="k">простой: <b style="color:var(--bad)">'+fmtDur(d.downMin)+'</b></div>'
    :((d.uptime!==null&&d.uptime<100)?'<div class="k">кратковременный сбой</div>':'<div class="k">сбоев нет</div>');
  var vpn=(d.vpnMin&&d.vpnMin>0)
    ?'<div class="k">на устройстве был VPN: <b style="color:var(--tx2)">'+fmtDur(d.vpnMin)+'</b></div>':'';
  tip.innerHTML='<div class="d">'+d.label+'</div><div class="k">аптайм: <b style="color:'+uc+'">'+ut+'</b></div>'+down+vpn;
  tip.style.opacity="1";moveTip(e);
}
function showTipServer(e,s){
  var u=s.uptime30;
  var uc=(u===null)?"var(--tx2)":(u>=99.9?"var(--ok)":(u>=99?"var(--warn)":"var(--bad)"));
  var ut=(u===null)?"нет данных":u.toFixed(2)+"%";
  var dm=(s.downMin30>0)?'<b style="color:var(--bad)">'+fmtDur(s.downMin30)+'</b>':'<b style="color:var(--ok)">0 мин</b>';
  // Статус: нет пробников / нет свежих данных (нет подтверждения) / онлайн / офлайн.
  var statusHtml;
  if(s.noData)statusHtml='<b style="color:var(--tx3)">нет пробников</b>';
  else if(!s.anyFresh)statusHtml='<b style="color:#3d7bf0">нет данных (никто не проверяет)</b>';
  else if(s.online)statusHtml='<b style="color:var(--ok)">онлайн</b>';
  else statusHtml='<b style="color:var(--bad)">офлайн</b>';
  tip.innerHTML='<div class="d">'+escapeHtml(s.name)+'</div>'+
    '<div class="k">статус: '+statusHtml+'</div>'+
    '<div class="k">аптайм 30 дн: <b style="color:'+uc+'">'+ut+'</b></div>'+
    '<div class="k">общий простой: '+dm+'</div>';
  tip.style.opacity="1";moveTip(e);
}
function renderToday(panel,data){
  var head=data.dayLabel||"Сегодня";
  if(!data.samples.length){
    panel.innerHTML='<div class="phead">'+head+'</div><div class="empty">Данных за этот день нет.</div>';
    panelH(panel);return;
  }
  var ds=data.dayStart,span=86400,W=1000;
  var chartTop=10,base=150,H=150,chartH=base-chartTop,mid=(chartTop+base)/2;
  var sw=Math.max(1.5,data.pollInterval/span*W);
  // Серые полосы VPN — рисуются ПОВЕРХ красных полос сбоев. Период «на
  // устройстве был включён VPN» важнее, чем «сбой»: мы в это время осознанно
  // не тестили, поэтому красный сбой под VPN перекрываем серым (red → gray).
  var vpnBands="";
  (data.vpnIntervals||[]).forEach(function(iv){
    var x1=(iv.start-ds)/span*W,x2=(iv.end-ds)/span*W;
    var w=Math.max(1.5,x2-x1);
    vpnBands+='<rect x="'+x1.toFixed(1)+'" y="0" width="'+w.toFixed(1)+'" height="'+H+'" fill="rgba(150,154,166,.42)"/>';
  });
  var bands="",runStart=null,runEnd=null,ptsRaw=[];
  function flush(){if(runStart!==null){bands+='<rect x="'+runStart.toFixed(1)+'" y="0" width="'+Math.max(1.5,runEnd-runStart).toFixed(1)+'" height="'+H+'" fill="rgba(232,80,80,.18)"/>';runStart=null;}}
  data.samples.forEach(function(s){
    var x=(s.ts-ds)/span*W;
    if(s.online){flush();if(s.latency>0)ptsRaw.push([x,s.latency]);}
    else{if(runStart===null)runStart=x;runEnd=x+sw;}
  });
  flush();
  function niceLo(v){var e=Math.pow(10,Math.floor(Math.log(v)/Math.LN10+1e-9)),m=v/e;return (m>=5?5:(m>=2?2:1))*e;}
  function niceHi(v){var e=Math.pow(10,Math.floor(Math.log(v)/Math.LN10+1e-9)),m=v/e;return (m<=1.0001?1:(m<=2.0001?2:(m<=5.0001?5:10)))*e;}
  var pmin=data.stats.pmin||50,pmax=data.stats.pmax||100;
  var lo=niceLo(Math.max(10,pmin)),hi=niceHi(Math.max(pmax,lo*4));
  var L0=Math.log(lo),LR=(Math.log(hi)-L0)||1;
  function yOf(v){var vv=v<lo?lo:(v>hi?hi:v);return base-(Math.log(vv)-L0)/LR*chartH;}
  var pts=ptsRaw.map(function(p){return [p[0],yOf(p[1])];});
  var ticks=[],t=lo,tg=0;
  while(t<=hi*1.0001&&tg++<40){ticks.push(t);var te=Math.pow(10,Math.floor(Math.log(t)/Math.LN10+1e-9)),tm=t/te;t=(tm<1.5?2:(tm<3.5?5:10))*te;}
  var grid="",yl="";
  ticks.forEach(function(tk){var ty=yOf(tk);grid+='<line x1="0" y1="'+ty.toFixed(1)+'" x2="1000" y2="'+ty.toFixed(1)+'" stroke="var(--line)" stroke-width="1"/>';yl+='<div class="tyaxis" style="top:'+ty.toFixed(1)+'px">'+tk+'</div>';});
  var poly=pts.map(function(p){return p[0].toFixed(1)+","+p[1].toFixed(1);});
  var line=pts.length?'<polyline points="'+poly.join(" ")+'" fill="none" stroke="#2f6bff" stroke-width="1.6" stroke-linejoin="round" stroke-linecap="round"/>':"";
  var area=pts.length?'<path d="M'+pts[0][0].toFixed(1)+','+base+' L'+poly.join(" L")+' L'+pts[pts.length-1][0].toFixed(1)+','+base+' Z" fill="url(#pgrad)"/>':"";
  var nowLine="";
  if(data.isToday){var nowX=((data.now-ds)/span*W);nowLine='<line x1="'+nowX.toFixed(1)+'" y1="0" x2="'+nowX.toFixed(1)+'" y2="'+base+'" stroke="var(--tx3)" stroke-width="1" stroke-dasharray="4 4"/>';}
  var svg='<svg viewBox="0 0 1000 '+H+'" preserveAspectRatio="none" class="tchart">'+
    '<defs><linearGradient id="pgrad" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#2f6bff" stop-opacity="0.20"/><stop offset="1" stop-color="#2f6bff" stop-opacity="0"/></linearGradient></defs>'+
    grid+area+bands+vpnBands+line+nowLine+
    '<line class="cursor" x1="0" y1="0" x2="0" y2="'+base+'" stroke="var(--tx2)" stroke-width="1" opacity="0"/>'+
    '</svg>';
  var zoom=2;
  var axis="";for(var h2=0;h2<=24;h2+=3){axis+='<span>'+("0"+h2).slice(-2)+':00</span>';}
  var st=data.stats,last=data.samples[data.samples.length-1];
  var stats='<div class="tstats">'+
    '<div><span>'+(data.isToday?"Проверок сегодня":"Проверок за день")+'</span><b>'+st.checks+'</b></div>'+
    '<div><span>Ошибок опроса</span><b style="color:'+(st.errors?"var(--bad)":"var(--ok)")+'">'+st.errors+'</b></div>'+
    '<div><span>'+(data.isToday?"Пинг сейчас":"Последний пинг")+'</span><b>'+(last.online?last.latency+" мс":"—")+'</b></div>'+
    '<div><span>Пинг мин / сред / макс</span><b>'+(st.pavg?st.pmin+" / "+st.pavg+" / "+st.pmax+" мс":"—")+'</b></div>'+
    '</div>';
  var cap='Пинг, мс · логарифмическая шкала · макс '+(data.stats.pmax||0)+' мс';
  var vpnHint=(data.vpnIntervals&&data.vpnIntervals.length)?', серые — VPN на устройстве':'';
  panel.innerHTML='<div class="phead">'+head+' · синяя линия — пинг, красные полосы — периоды сбоев'+vpnHint+'</div>'+
    stats+
    '<div class="tcaption">'+cap+'</div>'+
    '<div class="tchartwrap">'+
      '<div class="tscroll"><div class="tcanvas" style="width:'+(zoom*100)+'%">'+svg+'<div class="taxis">'+axis+'</div></div></div>'+
      yl+
    '</div>';
  var sc=panel.querySelector(".tscroll");
  if(sc){
    if(data.isToday){
      var nf=(data.now-ds)/span,tgt=nf*sc.scrollWidth-sc.clientWidth/2;
      sc.scrollLeft=Math.max(0,Math.min(tgt,sc.scrollWidth-sc.clientWidth));
    }else{sc.scrollLeft=0;}
  }
  var svgEl=panel.querySelector("svg"),cursor=svgEl.querySelector(".cursor"),samples=data.samples;
  function scrub(cx,ev){
    var r=svgEl.getBoundingClientRect();
    var frac=(cx-r.left)/r.width;if(frac<0)frac=0;if(frac>1)frac=1;
    var ts=ds+frac*span,best=null,bd=1e15;
    for(var i=0;i<samples.length;i++){var dd=Math.abs(samples[i].ts-ts);if(dd<bd){bd=dd;best=samples[i];}}
    if(!best)return;
    var x=(best.ts-ds)/span*1000;
    cursor.setAttribute("x1",x);cursor.setAttribute("x2",x);cursor.setAttribute("opacity","1");
    tip.innerHTML='<div class="d">'+fmtTime(best.ts,ds)+'</div>'+
      (best.online?'<div class="k">опрос: <b style="color:var(--ok)">успешно</b></div><div class="k">пинг: <b>'+best.latency+' мс</b></div>'
                  :'<div class="k"><b style="color:var(--bad)">ошибка опроса</b></div>');
    tip.style.opacity="1";moveTip(ev);
  }
  svgEl.addEventListener("mousemove",function(e){scrub(e.clientX,e);});
  svgEl.addEventListener("mouseleave",function(){cursor.setAttribute("opacity","0");hideTip();});
  var td=null;
  svgEl.addEventListener("touchstart",function(e){var t=e.touches[0];if(!t)return;td={x:t.clientX,y:t.clientY,m:false};scrub(t.clientX,t);},{passive:true});
  svgEl.addEventListener("touchmove",function(e){if(!td)return;var t=e.touches[0];if(!t)return;if(!td.m&&(Math.abs(t.clientX-td.x)>8||Math.abs(t.clientY-td.y)>8)){td.m=true;cursor.setAttribute("opacity","0");hideTip();}},{passive:true});
  svgEl.addEventListener("touchend",function(){td=null;},{passive:true});
  panelH(panel);
}
var built=false,order=[],nodes={};
function srvUpColor(u){return (u===null)?"var(--tx2)":(u>=99.9?"var(--ok)":(u>=99?"var(--warn)":"var(--bad)"));}
function updateTop(data){
  document.getElementById("title").textContent=data.title;
  document.getElementById("subtitle").textContent=data.subtitle;
  var t=data.totals;
  document.getElementById("s-online").textContent=t.online+" / "+t.total;
  document.getElementById("s-uptime").textContent=(t.uptime30===null?"—":t.uptime30.toFixed(2)+"%");
  document.getElementById("s-ping").textContent=(t.avgLatency?t.avgLatency+" ms":"—");
  var allUp=t.online===t.total&&t.total>0;
  var ov=document.getElementById("overall");
  ov.className="pill "+(allUp?"ok":"bad");
  ov.innerHTML='<span class="dot" style="background:currentColor"></span><span>'+
    (allUp?"Все системы в норме":(t.total-t.online)+" сервер(ов) недоступно")+'</span>';
  document.getElementById("foot").textContent=
    (data.lastCheck?"последняя проверка "+data.lastCheck:"ожидание первой проверки")+
    " · интервал "+Math.round(data.pollInterval/60*10)/10+" мин";
}
function panelH(panel){
  var it=panel.parentElement;
  if(it&&it.classList.contains("open"))panel.style.maxHeight=(panel.scrollHeight+40)+"px";
}
function _probeQ(name){return name?"&probeName="+encodeURIComponent(name):"";}
function openPanel(panel,sid,probeName){
  hideTip();
  if(!probeName){
    panel.innerHTML='<div class="empty">Кликни на полосу пробника, чтобы увидеть его график пинга.</div>';
    panelH(panel);return;
  }
  if(!panel.innerHTML.trim()||panel.querySelector(".empty"))panel.innerHTML='<div class="empty">Загрузка…</div>';
  fetch("api/today?sid="+encodeURIComponent(sid)+_probeQ(probeName))
    .then(function(r){return r.json();})
    .then(function(td){renderToday(panel,td);})
    .catch(function(){panel.innerHTML='<div class="empty">Не удалось загрузить.</div>';panelH(panel);});
}
function refreshPanel(panel,sid,probeName){
  if(!probeName)return;
  var sc=panel.querySelector(".tscroll");
  var keep=sc?sc.scrollLeft:null;
  fetch("api/today?sid="+encodeURIComponent(sid)+_probeQ(probeName))
    .then(function(r){return r.json();})
    .then(function(td){
      renderToday(panel,td);
      if(keep!==null){var s2=panel.querySelector(".tscroll");if(s2)s2.scrollLeft=keep;}
    })
    .catch(function(){});
}
function loadDay(panel,sid,date,probeName){
  hideTip();
  if(!probeName){
    panel.innerHTML='<div class="empty">Сначала выбери пробника (клик по его полосе).</div>';
    panelH(panel);return;
  }
  if(!panel.innerHTML.trim()||panel.querySelector(".empty"))panel.innerHTML='<div class="empty">Загрузка…</div>';
  fetch("api/day?sid="+encodeURIComponent(sid)+"&date="+encodeURIComponent(date)+_probeQ(probeName))
    .then(function(r){return r.json();})
    .then(function(td){renderToday(panel,td);})
    .catch(function(){panel.innerHTML='<div class="empty">Не удалось загрузить.</div>';panelH(panel);});
}
function applyServer(item,s,days){
  item._label._s=s;
  // Точка статуса сервера:
  //   серый  — пробников нет вообще;
  //   синий  — пробники есть, но сейчас никто не шлёт свежие данные →
  //            подтверждения работает/не работает НЕТ (не значит «офлайн»);
  //   зелёный — хотя бы один свежий пробник видит сервер онлайн;
  //   красный — есть свежие данные, и сервер офлайн.
  if(s.noData){item._dot.style.background="#cfd6df";}
  else if(!s.anyFresh){item._dot.style.background="#3d7bf0";}
  else if(s.online){item._dot.style.background="#16b07a";}
  else{item._dot.style.background="#e8504e";}
  item._p.textContent=(s.uptime30===null)?"—":s.uptime30.toFixed(2)+"%";
  item._p.style.color=srvUpColor(s.uptime30);
  var sub="";
  if(s.latencyMs)sub=s.latencyMs+" ms · ";
  sub+=days+" дн";
  item._s2.textContent=sub;
  applyBands(item,s,days);
}
function applyBands(item,s,days){
  // Контейнер полос-пробников под .row.
  var bands=item._bands;
  if(!bands){
    bands=document.createElement("div");bands.className="bands";
    if(item._panel)item.insertBefore(bands,item._panel);
    else item.appendChild(bands);
    item._bands=bands;
  }
  var regional=s.regional||[];
  if(!regional.length){
    bands.className="emptyBands";
    bands.textContent="Нет пробников. Поставь probe-агент в зоне блокировки — см. README.";
    return;
  }
  bands.className="bands";
  // Синкаем .band ноды с regional. Идентификатор полосы = ИМЯ пробника,
  // потому что несколько probe_id с одним именем мерджатся в одну полосу.
  var byName={};
  Array.prototype.forEach.call(bands.querySelectorAll(".band"),function(el){
    byName[el._pid]=el;
  });
  var seen={};
  regional.forEach(function(r,idx){
    seen[r.probe]=true;
    var b=byName[r.probe];
    if(!b){b=buildBand(item,r,s);bands.appendChild(b);}
    else updateBand(b,r);
    if(bands.children[idx]!==b)bands.insertBefore(b,bands.children[idx]||null);
  });
  Array.prototype.slice.call(bands.querySelectorAll(".band")).forEach(function(el){
    if(!seen[el._pid])el.remove();
  });
}
function buildBand(item,r,s){
  var b=document.createElement("div");b.className="band";b._pid=r.probe;b._probeName=r.probe;
  var nm=document.createElement("div");nm.className="bandName";
  var dot=document.createElement("span");dot.className="bandDot";
  var nmText=document.createElement("span");nmText.className="bandPName";
  nmText.textContent=r.probe;
  nm.appendChild(dot);nm.appendChild(nmText);
  var bars=document.createElementNS(SVGNS,"svg");bars.setAttribute("class","bandBars");
  bars.setAttribute("viewBox","0 0 1000 100");bars.setAttribute("preserveAspectRatio","none");
  var st=document.createElement("div");st.className="bandStat";
  var pct=document.createElement("div");pct.className="bandPct";
  var sub=document.createElement("div");sub.className="bandSub";
  st.appendChild(pct);st.appendChild(sub);
  // Полноширинная строка ошибки (под основным рядом) — показывает err целиком.
  var errLine=document.createElement("div");errLine.className="bandErr";errLine.style.display="none";
  b.appendChild(nm);b.appendChild(bars);b.appendChild(st);b.appendChild(errLine);
  b._dot=dot;b._bars=bars;b._pct=pct;b._sub=sub;b._err=errLine;b._rects=[];
  // Клик по полосе раскрывает панель именно для этого пробника (по имени —
  // на бэке мерджатся все probe_id с этим именем).
  b.addEventListener("click",function(e){
    e.stopPropagation();
    item._activeProbe=r.probe;
    item._day=null;
    if(!item.classList.contains("open")){
      item.classList.add("open");
    }
    openPanel(item._panel,item._sid,r.probe);
  });
  updateBand(b,r);
  return b;
}
function updateBand(b,r){
  // Точка статуса устройства: неактивно (синий) → VPN (серый) → онлайн
  // (зелёный) → офлайн (красный).
  if(!r.fresh)b._dot.className="bandDot bandDotStale";       // неактивно — синий
  else if(r.vpnActive)b._dot.className="bandDot bandDotVpn";  // VPN — серый
  else if(r.online)b._dot.className="bandDot bandDotOk";      // онлайн — зелёный
  else b._dot.className="bandDot bandDotBad";                 // офлайн — красный
  // SVG 30-дневная шкала.
  var days=r.days||[];
  var N=days.length||1;
  var slot=1000/N,gap=Math.min(7,slot*0.32);
  // Пересоздаём бары если число изменилось.
  if(b._rects.length!==N){
    while(b._bars.firstChild)b._bars.removeChild(b._bars.firstChild);
    b._rects=[];b._vpnRects=[];
    days.forEach(function(d,i){
      var rect=document.createElementNS(SVGNS,"rect");
      rect.setAttribute("x",(i*slot+gap/2).toFixed(2));rect.setAttribute("y","0");
      rect.setAttribute("width",(slot-gap).toFixed(2));rect.setAttribute("height","100");
      rect.setAttribute("rx","7");
      rect._d=d;
      rect.addEventListener("mouseenter",function(e){e.stopPropagation();showTipDay(e,this._d);});
      rect.addEventListener("mousemove",moveTip);
      rect.addEventListener("mouseleave",hideTip);
      rect.addEventListener("click",function(e){
        e.stopPropagation();
        var bandEl=this.parentNode.parentNode;
        var itemEl=bandEl.parentNode.parentNode;
        itemEl._activeProbe=bandEl._probeName||bandEl._pid;
        itemEl._day=this._d.date;
        if(!itemEl.classList.contains("open"))itemEl.classList.add("open");
        loadDay(itemEl._panel,itemEl._sid,this._d.date,bandEl._probeName||bandEl._pid);
      });
      b._bars.appendChild(rect);b._rects.push(rect);
      // Оверлей VPN — рисуется поверх бара дня. Высота rect'а
      // пропорциональна доле VPN-минут от суток (макс 100). Невидим, если
      // в этот день VPN не было.
      var vpn=document.createElementNS(SVGNS,"rect");
      vpn.setAttribute("class","vpn");
      vpn.setAttribute("x",(i*slot+gap/2).toFixed(2));vpn.setAttribute("y","0");
      vpn.setAttribute("width",(slot-gap).toFixed(2));
      vpn.setAttribute("rx","7");vpn.setAttribute("height","0");
      b._bars.appendChild(vpn);b._vpnRects.push(vpn);
    });
  }
  for(var i=0;i<N;i++){
    var d=days[i];if(!d)continue;
    b._rects[i]._d=d;
    b._rects[i].setAttribute("fill",colorFor(d));
    // VPN-доля от 24ч → высота оверлея (0..100). При vpnMin==0 высота 0 → скрыт.
    var vh=Math.min(100,Math.round(((d.vpnMin||0)/1440)*100));
    b._vpnRects[i].setAttribute("height",vh.toString());
  }
  // Текст справа.
  if(r.uptime30===null){
    b._pct.textContent="—";b._pct.style.color="var(--tx3)";
  }else{
    b._pct.textContent=r.uptime30.toFixed(2)+"%";
    b._pct.style.color=srvUpColor(r.uptime30);
  }
  var line="";var errFull="";
  if(!r.fresh){
    var ago=Math.floor((Date.now()/1000-(r.lastSeenTs||0))/60);
    line="нет данных "+(ago>0?ago+" мин":"только что");
  }else if(r.vpnActive){
    line="VPN включён";     // устройство сейчас в VPN — пробы не идут
  }else if(r.online&&r.latencyMs>0){
    line=r.latencyMs+" ms";
  }else if(!r.online&&r.err){
    line="ошибка";          // короткий тег справа
    errFull=r.err;          // полный текст — в отдельной строке ниже
    b.title=r.err;
  }else{
    line="—";
  }
  b._sub.textContent=line;
  // Полноширинная строка ошибки: показываем целиком, скрываем если ошибки нет.
  if(errFull){
    b._err.textContent=errFull;b._err.style.display="";
  }else{
    b._err.textContent="";b._err.style.display="none";
  }
}
function buildList(data){
  var list=document.getElementById("list");
  list.innerHTML="";nodes={};order=[];
  data.servers.forEach(function(s,idx){
    var item=document.createElement("div");item.className="item";item._sid=s.sid;
    item.style.animationDelay=Math.min(idx*0.04,0.5)+"s";
    var row=document.createElement("div");row.className="row";
    var flag=s.cc?'<img class="flag" src="https://flagcdn.com/'+s.cc+'.svg" alt="" loading="lazy">':'<span class="flag"></span>';
    var label=document.createElement("div");label.className="label";
    label.innerHTML=flag+'<div class="nm"><div class="name"><span class="sdot"></span><span>'+escapeHtml(s.name)+'</span></div></div>';
    label._s=s;
    label.addEventListener("mouseenter",function(e){showTipServer(e,this._s);});
    label.addEventListener("mousemove",moveTip);
    label.addEventListener("mouseleave",hideTip);
    var st2=document.createElement("div");st2.className="stat2";
    var pEl=document.createElement("div");pEl.className="p";
    var sEl=document.createElement("div");sEl.className="s";
    st2.appendChild(pEl);st2.appendChild(sEl);
    var chev=document.createElement("div");chev.className="chev";
    chev.innerHTML='<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 9l6 6 6-6"/></svg>';
    row.appendChild(label);row.appendChild(st2);row.appendChild(chev);
    if(adminMode){
      var del=document.createElement("button");del.type="button";del.className="delbtn";
      del.title="Удалить сервер";del.setAttribute("aria-label","Удалить сервер");
      del.innerHTML='<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6L6 18M6 6l12 12"/></svg>';
      del.addEventListener("click",function(e){e.stopPropagation();deleteServer(s.sid,s.name,s.members);});
      row.appendChild(del);
    }
    var panel=document.createElement("div");panel.className="panel";panel.innerHTML='<div class="empty">Загрузка…</div>';
    row.addEventListener("click",function(){
      if(item.classList.contains("open")){
        panel.style.maxHeight=panel.scrollHeight+"px";
        requestAnimationFrame(function(){item.classList.remove("open");panel.style.maxHeight="0px";});
      }else{
        item._day=null;
        // Открываем для первого пробника, если есть.
        var firstName=(s.regional&&s.regional[0])?s.regional[0].probe:"";
        item._activeProbe=firstName;
        item.classList.add("open");
        openPanel(panel,item._sid,firstName);
      }
    });
    item.appendChild(row);item.appendChild(panel);
    item._dot=label.querySelector(".sdot");item._p=pEl;item._s2=sEl;item._label=label;item._panel=panel;
    applyServer(item,s,data.days);
    list.appendChild(item);order.push(s.sid);nodes[s.sid]=item;
  });
  built=true;
}
function sameOrder(data){
  if(!built||order.length!==data.servers.length)return false;
  for(var i=0;i<order.length;i++)if(order[i]!==data.servers[i].sid)return false;
  return true;
}
var lastSeen=null;
function render(data){
  updateTop(data);
  var en=!!data.adminEnabled;
  if(en!==adminEnabled){adminEnabled=en;setLockUI();}
  var fresh=data.lastCheck!==lastSeen;lastSeen=data.lastCheck;
  if(sameOrder(data)){
    data.servers.forEach(function(s){var item=nodes[s.sid];if(item)applyServer(item,s,data.days);});
    if(fresh)for(var sid in nodes){
      var it=nodes[sid];
      if(it.classList.contains("open")&&(it._day===null||it._day===undefined)){
        refreshPanel(it._panel,it._sid,it._activeProbe||"");
      }
    }
  }else{
    buildList(data);
  }
}
function load(){
  fetch("api/summary").then(function(r){return r.json();}).then(function(d){
    render(d);
    if(adminMode)loadAdminSettings();
    else{var ab=document.getElementById("adminbar");if(ab)ab.hidden=true;}
  })
  .catch(function(){document.getElementById("list").innerHTML='<div class="skel">Не удалось загрузить данные</div>';});
}
var adminEnabled=false, adminMode=false, adminToken="";
try{adminToken=localStorage.getItem("sp-admin-token")||"";}catch(e){}
var LOCK_CLOSED='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="11" width="16" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/></svg>';
var LOCK_OPEN='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="11" width="16" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 7.5-2"/></svg>';
function setLockUI(){
  var b=document.getElementById("lock");if(!b)return;
  if(!adminEnabled){b.hidden=true;return;}
  b.hidden=false;
  b.innerHTML=adminMode?LOCK_OPEN:LOCK_CLOSED;
  if(adminMode){b.classList.add("lockon");b.title="Выйти из админ-режима";}
  else{b.classList.remove("lockon");b.title="Войти в админ-режим";}
}
function checkAdmin(cb){
  if(!adminToken){adminMode=false;setLockUI();if(cb)cb();return;}
  fetch("api/admin/check",{method:"POST",headers:{"X-Admin-Token":adminToken}})
    .then(function(r){
      adminMode=r.ok;
      if(!r.ok){adminToken="";try{localStorage.removeItem("sp-admin-token");}catch(e){}}
      setLockUI();if(cb)cb();
    })
    .catch(function(){adminMode=false;setLockUI();if(cb)cb();});
}
function promptAdminToken(){
  var t=window.prompt("Введите админ-токен:","");
  if(t===null)return;
  t=(t||"").trim();if(!t)return;
  fetch("api/admin/check",{method:"POST",headers:{"X-Admin-Token":t}})
    .then(function(r){
      if(r.ok){
        adminToken=t;adminMode=true;
        try{localStorage.setItem("sp-admin-token",t);}catch(e){}
        setLockUI();built=false;load();
      }else{
        window.alert("Неверный токен");
      }
    })
    .catch(function(){window.alert("Не удалось проверить токен");});
}
function logoutAdmin(){
  adminToken="";adminMode=false;
  try{localStorage.removeItem("sp-admin-token");}catch(e){}
  setLockUI();var ab=document.getElementById("adminbar");if(ab)ab.hidden=true;
  built=false;load();
}
var autocleanState=null, autocleanLocked=false;
function applyToggleUI(id,state){
  var t=document.getElementById(id);if(!t)return;
  if(state){t.classList.add("actogon");}else{t.classList.remove("actogon");}
  t.setAttribute("aria-checked",state?"true":"false");
}
function loadAdminSettings(){
  var ab=document.getElementById("adminbar");if(!ab)return;
  if(!adminMode){ab.hidden=true;return;}
  fetch("api/admin/settings",{headers:{"X-Admin-Token":adminToken}})
    .then(function(r){if(!r.ok){if(r.status===401)logoutAdmin();throw 0;}return r.json();})
    .then(function(s){
      // Авто-удаление устаревших записей
      autocleanState=!!s.autoclean;
      autocleanLocked=!!s.autocleanLocked;
      var sub=document.getElementById("ac-sub");
      var lbl=document.getElementById("adminlabel");
      if(autocleanLocked){
        sub.innerHTML='выключено через переменную <code>STALE_AFTER_HOURS=0</code>';
        lbl.classList.add("aclocked");
      }else{
        sub.innerHTML='через <b>'+s.staleHours+'</b> ч после исчезновения из подписки';
        lbl.classList.remove("aclocked");
      }
      document.getElementById("ac-toggle").disabled=autocleanLocked;
      applyToggleUI("ac-toggle",autocleanState);
      ab.hidden=false;
    })
    .catch(function(){});
}
function postSetting(key,toggleId,getState,setState,getLocked){
  if(!adminMode||getLocked())return;
  var newState=!getState();
  applyToggleUI(toggleId,newState);
  var body={};body[key]=newState;
  fetch("api/admin/settings",{method:"POST",
    headers:{"X-Admin-Token":adminToken,"Content-Type":"application/json"},
    body:JSON.stringify(body)})
    .then(function(r){
      if(r.ok){setState(newState);}
      else{applyToggleUI(toggleId,getState());if(r.status===401)logoutAdmin();}
    })
    .catch(function(){applyToggleUI(toggleId,getState());});
}
(function(){
  var ac=document.getElementById("ac-toggle");
  if(ac)ac.addEventListener("click",function(){
    postSetting("autoclean","ac-toggle",
      function(){return autocleanState;},function(v){autocleanState=v;},
      function(){return autocleanLocked;});
  });
  var wipe=document.getElementById("wipe-btn");
  if(wipe)wipe.addEventListener("click",function(){
    if(!adminMode)return;
    if(!window.confirm("Стереть всю историю опросов?\n\nГрафики и 30-дневный аптайм всех серверов обнулятся. Регистрации устройств и список серверов сохранятся — данные начнут копиться заново.\n\nЭто действие необратимо."))return;
    wipe.disabled=true;
    fetch("api/admin/wipe-history",{method:"POST",
      headers:{"X-Admin-Token":adminToken}})
      .then(function(r){
        wipe.disabled=false;
        if(r.ok){built=false;load();window.alert("История опросов очищена.");}
        else if(r.status===401){logoutAdmin();}
        else{window.alert("Не удалось очистить историю");}
      })
      .catch(function(){wipe.disabled=false;window.alert("Сетевая ошибка");});
  });
})();
function deleteServer(sid,name,members){
  var n=members||1;
  var extra=n>1?'\n\nЭто группа из '+n+' sub-серверов одного хоста — будут удалены все.':'';
  if(!window.confirm('Удалить запись «'+name+'»?'+extra+'\n\nНакопленная статистика по ней будет удалена. Если этот сервер ещё есть в подписке, при следующем sync он снова появится — поэтому удаление помогает в первую очередь чистить старые дубли, оставшиеся после смены конфига.'))return;
  fetch("api/admin/delete",{method:"POST",
    headers:{"X-Admin-Token":adminToken,"Content-Type":"application/json"},
    body:JSON.stringify({sid:sid})})
    .then(function(r){
      if(r.ok){built=false;load();}
      else if(r.status===401){window.alert("Сессия истекла. Войдите заново.");logoutAdmin();}
      else{window.alert("Не удалось удалить запись");}
    })
    .catch(function(){window.alert("Сетевая ошибка");});
}
(function(){
  var b=document.getElementById("lock");if(!b)return;
  b.addEventListener("click",function(){adminMode?logoutAdmin():promptAdminToken();});
})();
(function(){
  var SUN='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/></svg>';
  var MOON='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z"/></svg>';
  var SPARK='<svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor" stroke="none"><path d="M12 2c.4 4.5 2.6 6.7 7 7-4.4.3-6.6 2.5-7 7-.4-4.5-2.6-6.7-7-7 4.4-.3 6.6-2.5 7-7z"/></svg>';
  var CODE='<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 17l5-5-5-5"/><path d="M11 19h9"/></svg>';
  var NEXT={light:"dark",dark:"claude",claude:"claude-dark","claude-dark":"light"};
  var ICON={light:MOON,dark:SPARK,claude:CODE,"claude-dark":SUN};
  var NAMES={light:"светлая",dark:"тёмная",claude:"Claude","claude-dark":"Claude Code"};
  var btn=document.getElementById("theme-btn");if(!btn)return;
  function cur(){
    var t=document.documentElement.getAttribute("data-theme");
    if(t==="light"||t==="dark"||t==="claude"||t==="claude-dark")return t;
    return (window.matchMedia&&matchMedia("(prefers-color-scheme: dark)").matches)?"dark":"light";
  }
  function setIcon(){var c=cur();btn.innerHTML=ICON[c];btn.title="Тема: "+NAMES[c]+" → "+NAMES[NEXT[c]];}
  function apply(t){document.documentElement.setAttribute("data-theme",t);try{localStorage.setItem("sp-theme",t);}catch(e){}setIcon();}
  setIcon();
  btn.addEventListener("click",function(){apply(NEXT[cur()]);});
})();
window.addEventListener("resize",function(){
  var ps=document.querySelectorAll(".item.open .panel");
  for(var i=0;i<ps.length;i++)ps[i].style.maxHeight=ps[i].scrollHeight+"px";
});
checkAdmin(load);
setInterval(function(){if(!document.hidden)load();},60000);
document.addEventListener("visibilitychange",function(){if(!document.hidden)load();});
</script>
</body>
</html>"""


_UNIQ_TOKENS = ["tchartwrap", "tcaption", "tchart", "tcanvas", "tscroll",
                "tyaxis", "tstats", "taxis", "phead", "sdot",
                "overall", "pgrad",
                "delbtn", "lockon", "lock",
                "adminbar", "adminrow", "adminlabel", "actoggle", "actogon", "aclocked",
                "bands", "band", "bandName", "bandPName", "bandDot",
                "bandDotOk", "bandDotBad", "bandDotVpn", "bandDotStale", "bandBars",
                "bandStat", "bandPct", "bandSub", "bandErr", "emptyBands"]
_UNIQ_PREFIX = "c" + os.urandom(3).hex()


def _uniquify(s):
    for t in sorted(_UNIQ_TOKENS, key=len, reverse=True):
        s = re.sub(r'(?<![\w-])' + re.escape(t) + r'(?![\w-])', _UNIQ_PREFIX + t, s)
    return s


_TPL = _uniquify(INDEX_HTML)


def page_html():
    img = find_brand_image()
    if img:
        v = int(os.path.getmtime(img))
        fav = '<link rel="icon" href="/favicon.png?v=%d">' % v
        logo = ('<img src="/logo?v=%d" alt="" '
                'style="width:100%%;height:100%%;object-fit:cover;border-radius:inherit">' % v)
    else:
        fav = ""
        logo = DEFAULT_LOGO_SVG
    return (_TPL
            .replace("__TITLE__", TITLE)
            .replace("__SUBTITLE__", SUBTITLE)
            .replace("__DAYS__", str(DAYS))
            .replace("__FAVICON__", fav)
            .replace("__LOGO__", logo))


def _consteq(a, b):
    if len(a) != len(b):
        return False
    r = 0
    for x, y in zip(a, b):
        r |= ord(x) ^ ord(y)
    return r == 0


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def version_string(self):
        return SERVER_HEADER

    def _is_admin(self):
        if not ADMIN_TOKEN:
            return False
        tok = self.headers.get("X-Admin-Token", "") or ""
        return bool(tok) and _consteq(tok, ADMIN_TOKEN)

    def _read_json(self):
        try:
            n = int(self.headers.get("Content-Length", "0") or "0")
        except Exception:
            n = 0
        if n <= 0 or n > 65536:
            return {}
        try:
            raw = self.rfile.read(n).decode("utf-8")
            return json.loads(raw) if raw else {}
        except Exception:
            return {}

    def _send(self, code, body, ctype, cache=NO_CACHE):
        data = body.encode("utf-8") if isinstance(body, str) else body
        gz = False
        ae = self.headers.get("Accept-Encoding", "")
        compressible = ("text/" in ctype or "json" in ctype
                        or "javascript" in ctype or "svg" in ctype)
        if compressible and "gzip" in ae and len(data) >= 256:
            data = gzip.compress(data, 6)
            gz = True
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(data)))
        self.send_header("X-Robots-Tag", "noindex, nofollow")
        self.send_header("Cache-Control", cache)
        if gz:
            self.send_header("Content-Encoding", "gzip")
            self.send_header("Vary", "Accept-Encoding")
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        parsed = urlparse(self.path)
        path = parsed.path
        if path in ("/", "/index.html"):
            self._send(200, page_html(), "text/html; charset=utf-8")
        elif path == "/api/summary":
            self._send(200, json.dumps(build_summary(), ensure_ascii=False),
                       "application/json; charset=utf-8")
        elif path == "/api/today":
            qs = parse_qs(parsed.query)
            sid = (qs.get("sid") or [""])[0]
            probe_id = (qs.get("probe") or [""])[0]
            probe_name = (qs.get("probeName") or [""])[0]
            self._send(200,
                       json.dumps(build_today(sid, probe_id, probe_name),
                                  ensure_ascii=False),
                       "application/json; charset=utf-8")
        elif path == "/api/day":
            qs = parse_qs(parsed.query)
            sid = (qs.get("sid") or [""])[0]
            date = (qs.get("date") or [""])[0]
            probe_id = (qs.get("probe") or [""])[0]
            probe_name = (qs.get("probeName") or [""])[0]
            self._send(200,
                       json.dumps(build_day(sid, date, probe_id, probe_name),
                                  ensure_ascii=False),
                       "application/json; charset=utf-8")
        elif path in ("/favicon.ico", "/favicon.png", "/logo"):
            img = find_brand_image()
            if not img:
                self._send(404, "Not Found", "text/plain; charset=utf-8")
                return
            ext = os.path.splitext(img)[1].lower()
            try:
                with open(img, "rb") as f:
                    blob = f.read()
                self._send(200, blob, IMG_EXT.get(ext, "application/octet-stream"), STATIC_CACHE)
            except Exception:
                self._send(404, "Not Found", "text/plain; charset=utf-8")
        elif path.startswith("/fonts/"):
            fn = os.path.basename(path)
            fp = os.path.join(FONT_DIR, fn)
            if fn.endswith(".woff2") and os.path.isfile(fp):
                with open(fp, "rb") as f:
                    blob = f.read()
                self._send(200, blob, "font/woff2", STATIC_CACHE)
            else:
                self._send(404, "Not Found", "text/plain; charset=utf-8")
        elif path == "/api/admin/settings":
            if not self._is_admin():
                self._send(401, '{"error":"unauthorized"}',
                           "application/json; charset=utf-8")
                return
            ac = get_setting("autoclean", autoclean_default()) == "1"
            self._send(200, json.dumps({
                "autoclean": ac,
                "staleHours": STALE_AFTER_HOURS,
                "autocleanLocked": STALE_AFTER_HOURS <= 0,
            }, ensure_ascii=False), "application/json; charset=utf-8")
        elif path == "/api/admin/probes":
            if not self._is_admin():
                self._send(401, '{"error":"unauthorized"}',
                           "application/json; charset=utf-8")
                return
            self._send(200,
                       json.dumps({"probes": list_probes(),
                                   "freshMinutes": PROBE_FRESH_MINUTES},
                                  ensure_ascii=False),
                       "application/json; charset=utf-8")
        elif path == "/api/admin/probe-diag":
            # Диагностика: сходить на PROBE_SUBSCRIPTION_URL с набором популярных
            # User-Agent'ов и показать первые байты ответа + кол-во распарсенных
            # vless-таргетов. Помогает подобрать правильный UA для подписки.
            if not self._is_admin():
                self._send(401, '{"error":"unauthorized"}',
                           "application/json; charset=utf-8")
                return
            if not PROBE_SUBSCRIPTION_URL:
                self._send(503, '{"error":"PROBE_SUBSCRIPTION_URL не задан"}',
                           "application/json; charset=utf-8")
                return
            uas = [
                PROBE_SUBSCRIPTION_USER_AGENT,
                "v2rayN/6.40", "v2rayN/6.0", "v2rayNG/1.8.0",
                "Streisand/1.0", "Streisand/1.6",
                "clash-verge/1.5.0", "ClashX/1.95",
                "sing-box/1.8", "sing-box/1.10",
                "Shadowrocket/1.0", "Quantumult%20X/1.0",
                "Mozilla/5.0",
            ]
            seen = set()
            results = []
            for ua in uas:
                if ua in seen:
                    continue
                seen.add(ua)
                rec = {"ua": ua}
                try:
                    req = urllib.request.Request(
                        PROBE_SUBSCRIPTION_URL, headers={"User-Agent": ua})
                    with urllib.request.urlopen(req, timeout=15) as r:
                        body = r.read()
                        rec["status"] = r.status
                        rec["contentType"] = r.headers.get("Content-Type", "")
                        rec["bytes"] = len(body)
                        rec["headSample"] = body[:240].decode("utf-8", "replace")
                    # Прогоняем через ту же логику парсера.
                    text = body.decode("utf-8", "ignore").strip()
                    if ("vless://" not in text and "vmess://" not in text
                            and "trojan://" not in text):
                        try:
                            padded = text + "=" * (-len(text) % 4)
                            decoded = base64.b64decode(padded).decode(
                                "utf-8", "ignore")
                            if "vless://" in decoded:
                                text = decoded
                                rec["base64Decoded"] = True
                        except Exception:
                            pass
                    n_lines = sum(1 for ln in text.splitlines() if ln.strip())
                    n_vless = sum(1 for ln in text.splitlines()
                                  if ln.strip().startswith("vless://"))
                    parsed = []
                    for ln in text.splitlines():
                        t = _parse_vless_line(ln.strip())
                        if t:
                            parsed.append({"name": t["name"], "host": t["host"]})
                    rec["lines"] = n_lines
                    rec["vlessLines"] = n_vless
                    rec["parsedTargets"] = len(parsed)
                    rec["sampleTargets"] = parsed[:3]
                except Exception as e:
                    rec["error"] = str(e)
                results.append(rec)
            self._send(200,
                       json.dumps({"url": PROBE_SUBSCRIPTION_URL,
                                   "currentUA": PROBE_SUBSCRIPTION_USER_AGENT,
                                   "tries": results},
                                  ensure_ascii=False, indent=2),
                       "application/json; charset=utf-8")
        elif path == "/api/probe/targets":
            # Список таргетов для пробника. Аутентификация — X-Probe-Token.
            # Сервер тянет подписку (PROBE_SUBSCRIPTION_URL) с кешем
            # PROBE_TARGETS_TTL_MIN минут. Параметр ?force=1 — игнорировать
            # кеш и обновить сейчас (нужно когда владелец только что поменял
            # подписку и хочет, чтобы пробники подхватили без задержки).
            token = self.headers.get("X-Probe-Token", "") or ""
            p = find_probe_by_token(token)
            if not p:
                self._send(401, '{"error":"unauthorized"}',
                           "application/json; charset=utf-8")
                return
            if not PROBE_SUBSCRIPTION_URL:
                self._send(503,
                    '{"error":"PROBE_SUBSCRIPTION_URL не задан на сервере"}',
                    "application/json; charset=utf-8")
                return
            qs = parse_qs(parsed.query)
            force = (qs.get("force") or [""])[0] in ("1", "true", "yes")
            targets = get_probe_targets(force=force)
            self._send(200,
                       json.dumps({"targets": targets,
                                   "fetchedAt": _targets_cache["ts"],
                                   "ttlMinutes": PROBE_TARGETS_TTL_MIN,
                                   "forced": force},
                                  ensure_ascii=False),
                       "application/json; charset=utf-8")
        elif path == "/api/admin/probe-targets":
            # Админский force-refresh: вернуть свежий список + статус кеша.
            if not self._is_admin():
                self._send(401, '{"error":"unauthorized"}',
                           "application/json; charset=utf-8")
                return
            if not PROBE_SUBSCRIPTION_URL:
                self._send(503,
                    '{"error":"PROBE_SUBSCRIPTION_URL не задан"}',
                    "application/json; charset=utf-8")
                return
            qs = parse_qs(parsed.query)
            force = (qs.get("force") or [""])[0] in ("1", "true", "yes")
            targets = get_probe_targets(force=force)
            self._send(200,
                       json.dumps({"count": len(targets),
                                   "targets": targets,
                                   "fetchedAt": _targets_cache["ts"],
                                   "ttlMinutes": PROBE_TARGETS_TTL_MIN,
                                   "forced": force},
                                  ensure_ascii=False),
                       "application/json; charset=utf-8")
        elif path == "/health":
            self._send(200, "OK", "text/plain; charset=utf-8")
        else:
            self._send(404, "Not Found", "text/plain; charset=utf-8")

    def do_DELETE(self):
        parsed = urlparse(self.path)
        path = parsed.path
        # Удаление пробника по probe_id: DELETE /api/admin/probes/<probe_id>
        if path.startswith("/api/admin/probes/"):
            if not self._is_admin():
                self._send(401, '{"error":"unauthorized"}',
                           "application/json; charset=utf-8")
                return
            pid = path[len("/api/admin/probes/"):].strip("/")
            ok = delete_probe(pid)
            self._send(200,
                       json.dumps({"ok": True, "deleted": ok}, ensure_ascii=False),
                       "application/json; charset=utf-8")
        else:
            self._send(404, "Not Found", "text/plain; charset=utf-8")

    def do_POST(self):
        parsed = urlparse(self.path)
        path = parsed.path
        if path == "/api/admin/check":
            if not ADMIN_TOKEN:
                self._send(404, '{"error":"admin disabled"}',
                           "application/json; charset=utf-8")
                return
            if self._is_admin():
                self._send(200, '{"ok":true}',
                           "application/json; charset=utf-8")
            else:
                self._send(401, '{"ok":false}',
                           "application/json; charset=utf-8")
        elif path == "/api/admin/delete":
            if not self._is_admin():
                self._send(401, '{"error":"unauthorized"}',
                           "application/json; charset=utf-8")
                return
            body = self._read_json()
            sid = (body.get("sid") if isinstance(body, dict) else None) or ""
            if not sid:
                self._send(400, '{"error":"sid required"}',
                           "application/json; charset=utf-8")
                return
            ok = delete_server(sid)
            self._send(200,
                       json.dumps({"ok": True, "deleted": ok}, ensure_ascii=False),
                       "application/json; charset=utf-8")
        elif path == "/api/admin/wipe-history":
            # Полная очистка истории опросов (probe_samples/daily/vpn + legacy).
            # Регистрации пробников и список серверов сохраняются.
            if not self._is_admin():
                self._send(401, '{"error":"unauthorized"}',
                           "application/json; charset=utf-8")
                return
            wipe_history()
            self._send(200, '{"ok":true}', "application/json; charset=utf-8")
        elif path == "/api/admin/settings":
            if not self._is_admin():
                self._send(401, '{"error":"unauthorized"}',
                           "application/json; charset=utf-8")
                return
            body = self._read_json()
            if isinstance(body, dict) and "autoclean" in body:
                set_setting("autoclean", "1" if body["autoclean"] else "0")
            ac = get_setting("autoclean", autoclean_default()) == "1"
            self._send(200, json.dumps({"ok": True, "autoclean": ac},
                                       ensure_ascii=False),
                       "application/json; charset=utf-8")
        elif path == "/api/admin/probes":
            # Создать нового пробника. Возвращает probe_id + probe_token
            # (токен показывается ОДИН раз — потом только хеш).
            if not self._is_admin():
                self._send(401, '{"error":"unauthorized"}',
                           "application/json; charset=utf-8")
                return
            body = self._read_json() or {}
            name = body.get("name") or ""
            # mode: "merge" (default, переиспользует pid), "new", "replace".
            # Совместимость: старый параметр replace=true → mode="replace".
            mode = (body.get("mode") or "").lower()
            if not mode:
                mode = "replace" if body.get("replace") else "merge"
            probe = create_probe(name, mode=mode)
            self._send(200,
                       json.dumps({"ok": True, **probe}, ensure_ascii=False),
                       "application/json; charset=utf-8")
        elif path == "/api/admin/refresh-all":
            # One-shot обновление двух уровней кешей statuspage'a:
            #   1) инвалидация кеша probe-targets + перетягивание подписки;
            #   2) немедленный poll_once к xray-checker (новые имена попадают
            #      в current — пробники смогут на них маппить).
            # xray-checker — отдельный контейнер, его сам по этому эндпоинту
            # не рестартим. Если он ещё не подтянул новую подписку:
            #   `docker compose restart xray-checker`
            if not self._is_admin():
                self._send(401, '{"error":"unauthorized"}',
                           "application/json; charset=utf-8")
                return
            targets_count = 0
            targets_err = ""
            if PROBE_SUBSCRIPTION_URL:
                try:
                    targets_count = len(get_probe_targets(force=True))
                except Exception as e:
                    targets_err = str(e)[:200]
            poll_err = ""
            try:
                poll_once()
            except Exception as e:
                poll_err = str(e)[:200]
            with _lock, conn() as c:
                servers_in_current = c.execute(
                    "SELECT COUNT(*) FROM current").fetchone()[0]
                names = [r[0] for r in c.execute(
                    "SELECT DISTINCT name FROM current ORDER BY name").fetchall()]
            self._send(200,
                       json.dumps({
                           "ok": True,
                           "probeTargets": {
                               "count": targets_count,
                               "fetchedAt": _targets_cache["ts"],
                               "error": targets_err,
                           },
                           "pollOnce": {
                               "ok": not poll_err,
                               "error": poll_err,
                               "serversInCurrent": servers_in_current,
                               "names": names,
                           },
                           "note": ("если новых имён всё ещё не видно — "
                                    "xray-checker ещё не обновил свою подписку. "
                                    "На сервере: `docker compose restart xray-checker`"),
                       }, ensure_ascii=False),
                       "application/json; charset=utf-8")
        elif path == "/api/probe/report":
            # Приём отчёта от пробника. Аутентификация — X-Probe-Token.
            token = self.headers.get("X-Probe-Token", "") or ""
            p = find_probe_by_token(token)
            if not p:
                self._send(401, '{"error":"unauthorized"}',
                           "application/json; charset=utf-8")
                return
            probe_id, probe_name = p
            body = self._read_json()
            if not isinstance(body, dict):
                body = {}
            geo = (body.get("geo") or "").strip()
            # vpn=true → агент задетектил включённый VPN на устройстве, пробы
            # пропустил. Регистрируем VPN-период (без results), чтобы UI его
            # отметил серым на барах/графике и пользователь понимал, почему
            # в этом окне нет точек.
            if body.get("vpn") is True:
                n = save_probe_vpn(probe_id, geo)
                kind = "vpn"
            else:
                results = body.get("results") or []
                n = save_probe_report(probe_id, geo, results)
                kind = "report"
            self._send(200,
                       json.dumps({"ok": True, "saved": n, "kind": kind,
                                   "probe": {"id": probe_id, "name": probe_name}},
                                  ensure_ascii=False),
                       "application/json; charset=utf-8")
        else:
            self._send(404, "Not Found", "text/plain; charset=utf-8")


def main():
    os.makedirs(os.path.dirname(DB_PATH) or ".", exist_ok=True)
    init_db()
    threading.Thread(target=ensure_fonts, daemon=True).start()
    threading.Thread(target=poller, daemon=True).start()
    print("xray-status on :%d, subscription=%s, tz=%s"
          % (PORT, PROBE_SUBSCRIPTION_URL or "(unset!)", TZ_NAME), flush=True)
    ThreadingHTTPServer(("0.0.0.0", PORT), Handler).serve_forever()


if __name__ == "__main__":
    main()
