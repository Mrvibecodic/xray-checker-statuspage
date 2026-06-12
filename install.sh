#!/usr/bin/env bash
set -euo pipefail

IMAGE="ghcr.io/mrvibecodic/xray-checker-statuspage:latest"
INSTALL_DIR="/opt/xray-checker-statuspage"

c_g(){ printf "\033[32m%s\033[0m\n" "$*"; }
c_y(){ printf "\033[33m%s\033[0m\n" "$*"; }
c_r(){ printf "\033[31m%s\033[0m\n" "$*"; }
die(){ c_r "ОШИБКА: $*"; exit 1; }

ask(){
  local p="$1" d="${2:-}" v=""
  if [ -n "$d" ]; then read -rp "$p [$d]: " v </dev/tty || true; else read -rp "$p: " v </dev/tty || true; fi
  echo "${v:-$d}"
}
ask_yn(){
  local p="$1" d="${2:-n}" v=""
  read -rp "$p ($([ "$d" = y ] && echo "Y/n" || echo "y/N")): " v </dev/tty || true
  v="${v:-$d}"; [[ "$v" =~ ^([Yy]|[Yy][Ee][Ss]|[Дд]|[Дд][Аа])$ ]]
}
num(){ case "$1" in (''|*[!0-9]*) echo "$2";; (*) echo "$1";; esac; }
asks(){ local p="$1" v=""; read -rsp "$p: " v </dev/tty || true; echo >&2; printf '%s' "$v"; }

gen_token(){
  openssl rand -hex 24 2>/dev/null \
    || head -c 24 /dev/urandom 2>/dev/null | xxd -p -c 1000 2>/dev/null | tr -d '\n' \
    || head -c 64 /dev/urandom 2>/dev/null | base64 2>/dev/null | tr -dc 'A-Za-z0-9' | head -c 48
}

read_existing_admin_token(){
  [ -f "$1" ] || return 0
  awk -F= '/^[[:space:]]*-[[:space:]]*ADMIN_TOKEN=/{sub(/^[[:space:]]*-[[:space:]]*ADMIN_TOKEN=/,"");print;exit}' "$1" | tr -d '[:space:]'
}

# Вписывает строку `- ADMIN_TOKEN=<token>` в блок statuspage существующего
# docker-compose.yml — сразу после `- DB_PATH=/data/status.db`. Сохраняет отступ.
inject_admin_token(){
  local file="$1" token="$2" tmp
  grep -q -- '- DB_PATH=/data/status.db' "$file" || return 1
  tmp="$(mktemp)" || return 1
  awk -v t="$token" '
    {
      print
      if (!done && match($0, /^[[:space:]]*- DB_PATH=\/data\/status\.db[[:space:]]*$/)) {
        indent = $0
        sub(/-.*/, "", indent)
        printf "%s- ADMIN_TOKEN=%s\n", indent, t
        done = 1
      }
    }
  ' "$file" > "$tmp" && mv "$tmp" "$file"
}

write_admin_token_info(){
  local token="$1"
  if [ -f .install-info ]; then
    grep -v '^Админ-токен' .install-info > .install-info.tmp 2>/dev/null || true
    echo "Админ-токен (удаление записей со страницы): ${token}" >> .install-info.tmp
    mv .install-info.tmp .install-info
  else
    echo "Админ-токен (удаление записей со страницы): ${token}" > .install-info
  fi
}

[ "$(id -u)" -eq 0 ] || die "запусти от root:  sudo bash install.sh"

c_g "== xray-checker-statuspage =="

export DEBIAN_FRONTEND=noninteractive
if command -v apt-get >/dev/null 2>&1; then
  apt-get update -qq || true
  apt-get install -y -qq git curl ca-certificates openssl >/dev/null 2>&1 || true
else
  command -v git >/dev/null || die "нет git, поставь вручную"
fi

if ! command -v docker >/dev/null 2>&1; then
  c_y "Docker не найден — ставлю через get.docker.com…"
  curl -fsSL https://get.docker.com | sh || die "не удалось поставить Docker"
fi
if ! docker compose version >/dev/null 2>&1; then
  c_y "compose-плагин не найден — ставлю…"
  apt-get install -y -qq docker-compose-plugin >/dev/null 2>&1 || die "поставь docker compose вручную"
fi
systemctl enable --now docker >/dev/null 2>&1 || true

mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR"

if [ -f docker-compose.yml ]; then
  c_y "Найден существующий docker-compose.yml."
  if ! ask_yn "Перенастроить заново? (нет = обновить образ и перезапустить)" n; then
    EXISTING_TOKEN="$(read_existing_admin_token docker-compose.yml)"
    if [ -z "$EXISTING_TOKEN" ]; then
      c_y "В docker-compose.yml нет ADMIN_TOKEN — это новая фича для ручного удаления старых записей со страницы (иконка-замок в шапке)."
      if ask_yn "Сгенерировать и вписать ADMIN_TOKEN в docker-compose.yml?" y; then
        NEW_TOKEN="$(gen_token)"
        if [ -n "$NEW_TOKEN" ] && inject_admin_token docker-compose.yml "$NEW_TOKEN"; then
          write_admin_token_info "$NEW_TOKEN"
          c_g "ADMIN_TOKEN добавлен. Сохрани токен (он же в ${INSTALL_DIR}/.install-info):"
          echo "  $NEW_TOKEN"
        else
          c_y "Не удалось автоматически вписать ADMIN_TOKEN — добавь вручную в секцию statuspage:"
          echo "  - ADMIN_TOKEN=${NEW_TOKEN:-$(gen_token)}"
        fi
      fi
    fi
    docker compose pull && docker compose up -d
    c_g "Обновлено. Текущая страница работает на прежних настройках."
    exit 0
  fi
fi

echo
c_g "--- Настройка ---"
SUB_URL=""
while [ -z "$SUB_URL" ]; do
  SUB_URL="$(ask 'URL подписки (SUBSCRIPTION_URL)')"
  [ -n "$SUB_URL" ] || c_y "Подписка обязательна — вставь ссылку."
done
TITLE="$(ask 'Заголовок страницы' 'Статус серверов')"
SUBTITLE="$(ask 'Подзаголовок' 'Доступность серверов в реальном времени')"
INTERVAL="$(ask 'Интервал проверки, сек' '300')"
DAYS="$(ask 'Сколько дней истории показывать' '30')"
TZ_VAL="$(ask 'Часовой пояс' 'Europe/Moscow')"
MUSER="$(ask 'Логин админки/метрик checker' 'admin')"
MPASS_DEF="$(openssl rand -hex 16 2>/dev/null || echo change-me-$RANDOM)"
MPASS="$(ask 'Пароль админки/метрик checker' "$MPASS_DEF")"
PORT="$(ask 'Локальный порт страницы (127.0.0.1:PORT)' '8080')"
INTERVAL="$(num "$INTERVAL" 300)"; DAYS="$(num "$DAYS" 30)"; PORT="$(num "$PORT" 8080)"

ADMIN_TOKEN_PREV="$(read_existing_admin_token docker-compose.yml)"
if [ -n "$ADMIN_TOKEN_PREV" ]; then
  ADMIN_TOKEN="$ADMIN_TOKEN_PREV"; ADMIN_TOKEN_NEW=n
else
  ADMIN_TOKEN="$(gen_token)"; ADMIN_TOKEN_NEW=y
fi
[ -n "$ADMIN_TOKEN" ] || die "не удалось сгенерировать ADMIN_TOKEN (нет openssl/urandom/base64?)"

echo
c_y "nginx (официальный nginx.org) на портах 80/443. Если на сервере уже есть панель"
c_y "(FastPanel/ISPmanager и т.п.) — выбери НЕТ и направь её прокси на http://127.0.0.1:${PORT}."
USE_NGINX=n; CERT_MODE=0; DOMAIN=""; EMAIL=""; CF_AUTH=1; CF_EMAIL=""; CF_KEY=""; CF_TOKEN=""
if ask_yn "Поставить nginx и проксировать на домен?" n; then
  DOMAIN="$(ask 'Домен (A-запись/Cloudflare на этот сервер)')"
  DOMAIN="$(printf '%s' "$DOMAIN" | sed -E 's#^https?://##; s#/.*$##' | tr -d '[:space:]')"
  if [ -z "$DOMAIN" ]; then
    c_y "Домен не указан — пропускаю nginx. Сервис останется на http://127.0.0.1:${PORT}."
  else
    USE_NGINX=y
    echo "Сертификат HTTPS (обязателен):"
    echo "  1) HTTP-01 — проверка по 80 порту (порт открыт извне)"
    echo "  2) Cloudflare DNS — без открытия порта (нужен доступ к Cloudflare)"
    CERT_MODE="$(num "$(ask 'Выбор (1/2)' 1)" 1)"
    [ "$CERT_MODE" = 2 ] || CERT_MODE=1
    EMAIL="$(ask 'Email для Let'\''s Encrypt')"
    while [ -z "$EMAIL" ]; do c_y "Email обязателен для сертификата."; EMAIL="$(ask 'Email для Let'\''s Encrypt')"; done
    if [ "$CERT_MODE" = 2 ]; then
      echo "Доступ к Cloudflare:"
      echo "  1) API Token (рекомендуется; права Zone.DNS: Edit)"
      echo "  2) Global API Key + email аккаунта Cloudflare"
      CF_AUTH="$(num "$(ask 'Выбор (1/2)' 1)" 1)"
      if [ "$CF_AUTH" = 2 ]; then
        CF_EMAIL="$(ask 'Email аккаунта Cloudflare')"
        CF_KEY="$(asks 'Cloudflare Global API Key')"
        { [ -n "$CF_EMAIL" ] && [ -n "$CF_KEY" ]; } || die "Данные Cloudflare обязательны для DNS-проверки."
      else
        CF_TOKEN="$(asks 'Cloudflare API Token')"
        [ -n "$CF_TOKEN" ] || die "Cloudflare API Token обязателен."
      fi
    fi
  fi
fi

cat > docker-compose.yml <<EOF
services:
  xray-checker:
    image: kutovoys/xray-checker
    container_name: xray-checker
    restart: unless-stopped
    environment:
      - SUBSCRIPTION_URL=${SUB_URL}
      - METRICS_PROTECTED=true
      - METRICS_USERNAME=${MUSER}
      - METRICS_PASSWORD=${MPASS}
      - WEB_PUBLIC=true
      - PROXY_CHECK_INTERVAL=${INTERVAL}
    ports:
      - "127.0.0.1:2112:2112"

  statuspage:
    image: ${IMAGE}
    container_name: statuspage
    restart: unless-stopped
    depends_on:
      - xray-checker
    environment:
      - CHECKER_URL=http://xray-checker:2112
      - POLL_INTERVAL=${INTERVAL}
      - DAYS=${DAYS}
      - TZ=${TZ_VAL}
      - TITLE=${TITLE}
      - SUBTITLE=${SUBTITLE}
      - DB_PATH=/data/status.db
      - ADMIN_TOKEN=${ADMIN_TOKEN}
    volumes:
      - ./data:/data
    ports:
      - "127.0.0.1:${PORT}:8080"
EOF

mkdir -p data
c_g "Тяну образ и запускаю контейнеры…"
docker compose pull || c_y "Не удалось стянуть образ statuspage. Если GHCR-пакет приватный — сделай его публичным (GitHub → Packages → этот пакет → Package settings → Change visibility → Public), либо выполни: docker login ghcr.io"
docker compose up -d || die "не удалось поднять контейнеры. Частая причина — занят порт ${PORT} или 2112 (старый сервис?). Освободи порт (или укажи другой при повторном запуске) и попробуй снова. Логи: docker compose logs"

sleep 6
if curl -fsS "http://127.0.0.1:2112/api/v1/public/proxies" >/dev/null 2>&1; then
  c_g "checker отвечает, подписка читается."
else
  c_y "checker пока не отвечает — возможно ещё стартует или подписка недоступна. Это не критично, проверь позже: docker compose logs xray-checker"
fi

CERT_OK=n
if [ "$USE_NGINX" = y ]; then
  if ! command -v nginx >/dev/null 2>&1; then
    c_y "Ставлю nginx из официального репозитория nginx.org…"
    apt-get install -y -qq curl gnupg2 ca-certificates lsb-release >/dev/null 2>&1 || true
    install -m 0755 -d /usr/share/keyrings 2>/dev/null || true
    curl -fsSL https://nginx.org/keys/nginx_signing.key | gpg --dearmor 2>/dev/null | tee /usr/share/keyrings/nginx-archive-keyring.gpg >/dev/null || true
    OS_ID="$(. /etc/os-release; echo "${ID:-ubuntu}")"; [ "$OS_ID" = debian ] && NGX_OS=debian || NGX_OS=ubuntu
    CODENAME="$(lsb_release -cs 2>/dev/null || true)"
    [ -z "$CODENAME" ] && CODENAME="$(. /etc/os-release; echo "${VERSION_CODENAME:-}")"
    echo "deb [signed-by=/usr/share/keyrings/nginx-archive-keyring.gpg] http://nginx.org/packages/${NGX_OS} ${CODENAME} nginx" > /etc/apt/sources.list.d/nginx.list
    printf 'Package: *\nPin: origin nginx.org\nPin: release o=nginx\nPin-Priority: 900\n' > /etc/apt/preferences.d/99nginx
    apt-get update -qq || true
    apt-get install -y -qq nginx >/dev/null 2>&1 || true
    sed -i 's/^user[[:space:]].*/user www-data;/' /etc/nginx/nginx.conf 2>/dev/null || true
    systemctl enable --now nginx >/dev/null 2>&1 || true
  fi
  if ! command -v nginx >/dev/null 2>&1; then
    c_y "Не удалось поставить nginx — пропускаю. Сервис работает на http://127.0.0.1:${PORT}."
    USE_NGINX=n
  else
    mkdir -p /etc/nginx/conf.d /var/www/acme/.well-known/acme-challenge
    CONF="/etc/nginx/conf.d/${DOMAIN}.conf"
    proxy_loc(){ cat <<NGX
    location / {
        proxy_pass http://127.0.0.1:${PORT};
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }
NGX
}
    write_http(){ cat > "$CONF" <<NGX
server {
    listen 80;
    server_name ${DOMAIN};
    location ^~ /.well-known/acme-challenge/ { root /var/www/acme; }
$(proxy_loc)
}
NGX
}
    write_https(){ cat > "$CONF" <<NGX
server {
    listen 80;
    server_name ${DOMAIN};
    location ^~ /.well-known/acme-challenge/ { root /var/www/acme; }
    location / { return 301 https://\$host\$request_uri; }
}
server {
    listen 443 ssl;
    http2 on;
    server_name ${DOMAIN};
    ssl_certificate     /etc/letsencrypt/live/${DOMAIN}/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/${DOMAIN}/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    add_header Strict-Transport-Security "max-age=31536000" always;
$(proxy_loc)
}
NGX
}
    rm -f /etc/nginx/conf.d/default.conf 2>/dev/null || true
    write_http
    nginx -t >/dev/null 2>&1 && { systemctl reload nginx 2>/dev/null || systemctl restart nginx 2>/dev/null || true; }
    if [ "$CERT_MODE" != 0 ]; then
      apt-get install -y -qq certbot >/dev/null 2>&1 || true
      [ "$CERT_MODE" = 2 ] && { apt-get install -y -qq python3-certbot-dns-cloudflare >/dev/null 2>&1 || true; }
      if command -v certbot >/dev/null 2>&1; then
        ISSUED=n
        if [ "$CERT_MODE" = 2 ]; then
          umask 077
          if [ "$CF_AUTH" = 2 ]; then
            printf 'dns_cloudflare_email = %s\ndns_cloudflare_api_key = %s\n' "$CF_EMAIL" "$CF_KEY" > /root/.cloudflare.ini
          else
            printf 'dns_cloudflare_api_token = %s\n' "$CF_TOKEN" > /root/.cloudflare.ini
          fi
          chmod 600 /root/.cloudflare.ini
          certbot certonly --non-interactive --agree-tos -m "$EMAIL" \
            --dns-cloudflare --dns-cloudflare-credentials /root/.cloudflare.ini \
            --dns-cloudflare-propagation-seconds 30 --deploy-hook "systemctl reload nginx" -d "$DOMAIN" && ISSUED=y || true
        else
          certbot certonly --non-interactive --agree-tos -m "$EMAIL" \
            --webroot -w /var/www/acme --deploy-hook "systemctl reload nginx" -d "$DOMAIN" && ISSUED=y || true
        fi
        if [ "$ISSUED" = y ] && [ -f "/etc/letsencrypt/live/${DOMAIN}/fullchain.pem" ]; then
          write_https
          if nginx -t >/dev/null 2>&1; then systemctl reload nginx 2>/dev/null || systemctl restart nginx 2>/dev/null || true; CERT_OK=y; c_g "HTTPS включён для ${DOMAIN}."; else c_y "nginx -t ошибка после выпуска серта — оставил HTTP."; write_http; systemctl reload nginx 2>/dev/null || true; fi
        else
          c_y "Сертификат не выпущен (проверь 80 порт либо доступ Cloudflare). nginx работает по HTTP, можно повторить позже."
        fi
      else
        c_y "certbot не установился — nginx по HTTP."
      fi
    else
      c_g "nginx настроен на ${DOMAIN} (HTTP)."
    fi
  fi
fi

if [ "$USE_NGINX" = y ]; then
  SCHEME=http; [ "$CERT_OK" = y ] && SCHEME=https
  PUB_URL="${SCHEME}://${DOMAIN}"
else
  PUB_URL="http://127.0.0.1:${PORT}"
fi

cat > .install-info <<EOF
Логин/пароль админки checker: ${MUSER} / ${MPASS}
Админ-токен (удаление записей со страницы): ${ADMIN_TOKEN}
Страница: ${PUB_URL}
Локально: http://127.0.0.1:${PORT}
EOF

echo
c_g "================ Готово ================"
if [ "$USE_NGINX" = y ]; then
  c_g "Открой: ${PUB_URL}"
else
  c_g "Страница слушает: http://127.0.0.1:${PORT}"
  c_y "Направь на неё свою панель/прокси (proxy_pass http://127.0.0.1:${PORT})."
fi
echo "Админка/метрики checker под Basic Auth: ${MUSER} / ${MPASS}"
echo "Админ-токен страницы (иконка-замок в шапке): ${ADMIN_TOKEN}"
if [ "${ADMIN_TOKEN_NEW:-n}" = y ]; then
  c_g "  ↑ сгенерирован автоматически — сохрани, чтобы войти в админ-режим в браузере."
fi
echo "Доступы сохранены в ${INSTALL_DIR}/.install-info"
echo "Фавикон/лого: положи файл favicon.png в ${INSTALL_DIR}/data — подхватится сам."
