#!/usr/bin/env python3
"""
xray-checker-statuspage probe agent (TLS handshake, MVP)

Что делает:
  1. Раз в INTERVAL секунд тянет список таргетов с сервера статус-страницы
     (GET /api/probe/targets под X-Probe-Token) — сервер сам парсит
     подписку и отдаёт уже разобранные {name, host, port, sni}.
  2. Для каждого таргета делает TCP+TLS handshake к host:port с правильным SNI.
  3. Отправляет результаты на STATUSPAGE_URL/api/probe/report.

Почему агент НЕ тянет подписку напрямую: подписка обычно лежит за VPN,
агенту в зоне блокировки она не доступна без туннеля. Сервер в облаке
тянет её и отдаёт уже распарсенной. Бонусом — секретный URL подписки не
светится на устройстве пользователя.

В начале каждого цикла проверяет свою геолокацию (ifconfig.co/country-iso).
Если страна не совпадает с EXPECT_COUNTRY, в лог пишется warning — но
репорт всё равно уходит (только с пометкой `geo` для UI).

Конфиг — через переменные окружения:
  STATUSPAGE_URL     обязательно   куда репортить (https://status.example.com)
  PROBE_TOKEN        обязательно   токен пробника (выдаётся при регистрации)
  INTERVAL           60            секунды между циклами
  TIMEOUT            10            таймаут TCP+TLS handshake (сек)
  EXPECT_COUNTRY     RU            ожидаемая страна пробника
  GEO_CHECK_URL      https://ifconfig.co/country-iso

Запуск:
  STATUSPAGE_URL=https://... PROBE_TOKEN=... python3 agent.py
"""
import json
import os
import socket
import ssl
import sys
import time
import urllib.error
import urllib.request
import uuid as _uuid

STATUSPAGE_URL = os.environ.get("STATUSPAGE_URL", "").rstrip("/")
PROBE_TOKEN = os.environ.get("PROBE_TOKEN", "").strip()
INTERVAL = int(os.environ.get("INTERVAL", "60"))
TIMEOUT = float(os.environ.get("TIMEOUT", "10"))
EXPECT_COUNTRY = os.environ.get("EXPECT_COUNTRY", "RU").strip().upper()
GEO_CHECK_URL = os.environ.get("GEO_CHECK_URL", "https://ifconfig.co/country-iso")
USER_AGENT = "xrs-probe/0.1 (+macos)"


def log(*a):
    print("[probe]", *a, file=sys.stdout, flush=True)


def err(*a):
    print("[probe:err]", *a, file=sys.stderr, flush=True)


def http_get(url, timeout=10, insecure=False):
    req = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
    ctx = None
    if insecure and url.startswith("https://"):
        ctx = ssl.create_default_context()
        ctx.check_hostname = False
        ctx.verify_mode = ssl.CERT_NONE
    with urllib.request.urlopen(req, timeout=timeout, context=ctx) as r:
        return r.read()


def fetch_geo():
    """Возвращает двухбуквенный код страны нашего IP (e.g. 'RU') или '' если не удалось.
    На macOS без установленных CA-сертификатов системный TLS-verify падает —
    делаем fallback на unverified context (для geo-check это безопасно, нам не
    нужна гарантия подлинности этого ответа)."""
    for insecure in (False, True):
        try:
            data = http_get(GEO_CHECK_URL, timeout=8,
                            insecure=insecure).decode("utf-8", "ignore").strip().upper()
            return data[:2] if data else ""
        except Exception as e:
            if insecure:
                err("geo check failed:", e)
                return ""


def fetch_targets():
    """GET /api/probe/targets с X-Probe-Token. Возвращает список {name,host,port,sni}."""
    url = STATUSPAGE_URL + "/api/probe/targets"
    req = urllib.request.Request(
        url, headers={"User-Agent": USER_AGENT, "X-Probe-Token": PROBE_TOKEN})
    try:
        with urllib.request.urlopen(req, timeout=20) as r:
            data = json.loads(r.read().decode("utf-8", "ignore"))
        return data.get("targets") or []
    except urllib.error.HTTPError as e:
        if e.code == 503:
            raise RuntimeError(
                "сервер: PROBE_SUBSCRIPTION_URL не задан (или пуст). "
                "Добавь его в docker-compose.yml в env statuspage и перезапусти.")
        if e.code == 401:
            raise RuntimeError(
                "сервер: токен пробника отвергнут. Возможно, пробника удалили — "
                "переустанови через install-macos.sh.")
        raise RuntimeError("сервер ответил HTTP %d" % e.code)
    except urllib.error.URLError as e:
        raise RuntimeError(
            "сеть: не достучаться до %s — %s. Проверь, что STATUSPAGE_URL "
            "правильный (открой в браузере — должна быть видна статус-страница)."
            % (STATUSPAGE_URL, e.reason))


def _open_tls(host, port, sni):
    """Открывает TLS-соединение к host:port с указанным SNI. Возвращает
    (sock, rtt_ms) или бросает исключение."""
    t0 = time.monotonic()
    sock = socket.create_connection((host, port), timeout=TIMEOUT)
    ctx = ssl.create_default_context()
    ctx.check_hostname = False
    ctx.verify_mode = ssl.CERT_NONE
    try:
        ctx.set_alpn_protocols(["h2", "http/1.1"])
    except (NotImplementedError, AttributeError):
        pass
    tls = ctx.wrap_socket(sock, server_hostname=sni)
    rtt = int((time.monotonic() - t0) * 1000)
    return tls, rtt


def _build_vless_request(uuid_str, dest_host="www.cloudflare.com",
                         dest_port=443):
    """Минимальный VLESS-request (без addons).
    Структура:
      version(1) | UUID(16) | addonsLen(1) | command(1) | port(2 BE) |
      addrType(1=ipv4|2=domain|3=ipv6) | addr(N).
    """
    uuid_bytes = _uuid.UUID(uuid_str).bytes
    return (b"\x00" + uuid_bytes + b"\x00"          # version + UUID + no addons
            + b"\x01"                                # command: TCP
            + int(dest_port).to_bytes(2, "big")
            + b"\x02"                                # address type: domain
            + len(dest_host).to_bytes(1, "big")
            + dest_host.encode("ascii"))


def tls_probe(host, port, sni):
    """Только TCP+TLS handshake. Для REALITY этого недостаточно (любой клиент
    проходит handshake — REALITY-fallback прокидывает «чужих» на безопасный
    домен). Используется как fallback когда нет UUID."""
    sock = None
    try:
        sock, rtt = _open_tls(host, port, sni)
        try:
            sock.close()
        except Exception:
            pass
        return True, rtt, ""
    except Exception as e:
        try:
            if sock:
                sock.close()
        except Exception:
            pass
        return False, 0, type(e).__name__ + ": " + str(e)[:160]


def vless_probe(host, port, sni, uuid_str, security="tls"):
    """TCP+TLS handshake + VLESS-handshake с UUID + HTTP-ping через туннель.
    Это валидирует конкретный конфиг (UUID/pbk/shortId), а не только TLS-
    достижимость хоста.

    Sequence:
      1. TLS handshake к host:port с SNI.
      2. Шлём VLESS-request (UUID + target cp.cloudflare.com:80) и сразу
         HTTP/1.0 GET через туннель.
      3. Читаем response. Должно быть:
         - первые 2 байта: VLESS-response (0x00 0x00 = version + no addons),
         - дальше где-то "HTTP/" — значит туннель действительно прокинул нас
           на cloudflare и оттуда вернулся HTTP-ответ.
      4. Если VLESS-response отсутствует или после него нет HTTP-данных —
         сервер не туннелирует (UUID битый, REALITY-fallback и т.д.).

    Cloudflare возвращает H2-фреймы (например, GOAWAY начинается с 0x00 0x00),
    которые случайно совпадают с VLESS-response — поэтому проверка HTTP-маркера
    обязательна, недостаточно только 2 байт.
    """
    if not uuid_str:
        return tls_probe(host, port, sni)
    t0 = time.monotonic()
    sock = None
    try:
        sock, _ = _open_tls(host, port, sni)
        sock.settimeout(min(TIMEOUT, 5.0))
        try:
            req = _build_vless_request(uuid_str, "cp.cloudflare.com", 80)
        except (ValueError, TypeError) as e:
            return False, 0, "Bad UUID: " + str(e)[:80]
        # Шлём VLESS-handshake + HTTP-запрос (через туннель) одним сокетом.
        http_req = (b"GET /cdn-cgi/trace HTTP/1.0\r\n"
                    b"Host: cp.cloudflare.com\r\n"
                    b"User-Agent: xrs-probe\r\n\r\n")
        sock.sendall(req + http_req)
        # Читаем до 4 KB, ищем HTTP-маркер или таймаут.
        buf = b""
        deadline = time.monotonic() + min(TIMEOUT, 6.0)
        while time.monotonic() < deadline and len(buf) < 4096:
            try:
                chunk = sock.recv(4096 - len(buf))
            except socket.timeout:
                break
            if not chunk:
                break
            buf += chunk
            if b"HTTP/1." in buf:
                break
        rtt = int((time.monotonic() - t0) * 1000)
        if not buf:
            tag = "REALITY fallback или auth fail" if security == "reality" \
                  else "EOF после VLESS-handshake"
            return False, 0, tag
        # Должны увидеть VLESS-response (0x00 0x00) + HTTP-ответ от cloudflare.
        if len(buf) < 2 or buf[0] != 0x00 or buf[1] != 0x00:
            return False, 0, "no VLESS response: " + buf[:24].hex()
        if b"HTTP/1." not in buf[2:]:
            # VLESS-handshake вроде принят, но туннель не прошёл к cloudflare.
            # Скорее всего REALITY-fallback который случайно начал ответ с
            # 00 00 (H2 frame header) — и при этом HTTP не виден.
            return False, 0, "VLESS-handshake принят, но туннель не работает"
        return True, rtt, ""
    except (socket.timeout, ssl.SSLError, ConnectionResetError,
            ConnectionRefusedError, OSError) as e:
        return False, 0, type(e).__name__ + ": " + str(e)[:160]
    except Exception as e:
        return False, 0, "Exception: " + str(e)[:160]
    finally:
        try:
            if sock:
                sock.close()
        except Exception:
            pass


def report(geo, results):
    body = json.dumps({"geo": geo.lower(), "results": results},
                      ensure_ascii=False).encode("utf-8")
    req = urllib.request.Request(
        STATUSPAGE_URL + "/api/probe/report",
        data=body, method="POST",
        headers={
            "Content-Type": "application/json; charset=utf-8",
            "X-Probe-Token": PROBE_TOKEN,
            "User-Agent": USER_AGENT,
        })
    with urllib.request.urlopen(req, timeout=15) as r:
        return json.loads(r.read().decode("utf-8", "ignore"))


def one_cycle():
    geo = fetch_geo()
    # VPN-страж: если geo определилось и НЕ совпадает с EXPECT_COUNTRY —
    # на устройстве с большой вероятностью включён VPN, пробы пойдут через
    # туннель и дадут искажённую картину. Не отправляем такой цикл вообще.
    # Если geo не определилось (timeout/ошибка ifconfig.co) — отправляем как
    # обычно, доверяем пользователю (иначе при первых сбоях сети пробник
    # вообще ничего не пришлёт).
    if EXPECT_COUNTRY and geo and geo != EXPECT_COUNTRY:
        log("VPN detected: geo=%s, ожидалось %s — цикл пропущен, отчёт не отправлен"
            % (geo, EXPECT_COUNTRY))
        return
    try:
        targets = fetch_targets()
    except Exception as e:
        err("targets fetch failed:", e)
        return
    if not targets:
        err("сервер вернул пустой список таргетов")
        return
    results = []
    n_ok = 0
    for t in targets:
        host = t.get("host"); port = int(t.get("port") or 443)
        sni = t.get("sni") or host; name = t.get("name") or host
        uuid_str = (t.get("uuid") or "").strip()
        security = (t.get("security") or "").lower()
        if not host:
            continue
        # Если есть UUID — делаем полный VLESS-handshake (валидирует конфиг,
        # а не просто достижимость хоста). REALITY-fallback так отсеется.
        # Если UUID не передан (старый сервер без обновлённого парсера) —
        # фоллбек на TLS-only.
        if uuid_str:
            ok, rtt, errmsg = vless_probe(host, port, sni, uuid_str, security)
        else:
            ok, rtt, errmsg = tls_probe(host, port, sni)
        if ok:
            n_ok += 1
        results.append({"name": name, "ok": ok, "rtt": rtt, "err": errmsg})
    try:
        resp = report(geo, results)
        log("cycle: %d тестов, %d ok, geo=%s, saved=%s"
            % (len(results), n_ok, geo, resp.get("saved")))
    except Exception as e:
        err("report failed:", e)


def main():
    missing = [n for n, v in [
        ("STATUSPAGE_URL", STATUSPAGE_URL),
        ("PROBE_TOKEN", PROBE_TOKEN),
    ] if not v]
    if missing:
        err("обязательные env пусты:", ", ".join(missing))
        sys.exit(2)
    log("start: interval=%ds, target=%s, geo-expected=%s"
        % (INTERVAL, STATUSPAGE_URL, EXPECT_COUNTRY))
    while True:
        try:
            one_cycle()
        except Exception as e:
            err("unexpected:", e)
        time.sleep(INTERVAL)


if __name__ == "__main__":
    main()
