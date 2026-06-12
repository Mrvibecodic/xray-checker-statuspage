#!/usr/bin/env bash
# Установщик probe-агента xray-checker-statuspage для macOS.
# Регистрирует пробник на сервере, кладёт agent.py в ~/.xrs-probe/,
# создаёт LaunchAgent для авто-запуска при логине.
#
# Использование:
#   bash install-macos.sh           # интерактивный режим
#   STATUSPAGE_URL=... ADMIN_TOKEN=... bash install-macos.sh
#   bash install-macos.sh --uninstall
#
# Подписку (URL прокси-конфигов) задаёт владелец сервера через env
# PROBE_SUBSCRIPTION_URL — на устройство пользователя она НЕ попадает.
set -euo pipefail

LABEL="com.xrs.probe"
APP_DIR="${HOME}/.xrs-probe"
LOG_FILE="${HOME}/Library/Logs/xrs-probe.log"
PLIST="${HOME}/Library/LaunchAgents/${LABEL}.plist"
PYTHON="/usr/bin/python3"
AGENT_RAW_URL="https://raw.githubusercontent.com/Mrvibecodic/xray-checker-statuspage/main/probes/agent.py"

c_g(){ printf "\033[32m%s\033[0m\n" "$*"; }
c_y(){ printf "\033[33m%s\033[0m\n" "$*"; }
c_r(){ printf "\033[31m%s\033[0m\n" "$*"; }
die(){ c_r "ОШИБКА: $*"; exit 1; }

uninstall(){
  c_y "Удаление…"
  if launchctl print "gui/$(id -u)/${LABEL}" >/dev/null 2>&1; then
    launchctl bootout "gui/$(id -u)/${LABEL}" 2>/dev/null || true
  fi
  launchctl unload "${PLIST}" 2>/dev/null || true
  rm -f "${PLIST}"
  rm -rf "${APP_DIR}"
  if [ -L /usr/local/bin/monitorvpn ]; then
    rm -f /usr/local/bin/monitorvpn 2>/dev/null \
      || c_y "не могу удалить /usr/local/bin/monitorvpn без sudo (sudo rm -f /usr/local/bin/monitorvpn)"
  fi
  c_g "✓ удалено. Логи остались в ${LOG_FILE} (можешь удалить вручную)."
  exit 0
}

[ "${1:-}" = "--uninstall" ] && uninstall

[ "$(uname)" = "Darwin" ] || die "этот установщик только для macOS."
command -v "${PYTHON}" >/dev/null 2>&1 || die "не найден ${PYTHON}. Установи Xcode CLT: xcode-select --install"

ask(){ local p="$1" d="${2:-}" v=""; if [ -n "$d" ]; then read -rp "$p [$d]: " v; else read -rp "$p: " v; fi; echo "${v:-$d}"; }
asks(){ local p="$1" v=""; read -rsp "$p: " v; echo >&2; printf '%s' "$v"; }

c_g "== xray-checker-statuspage probe agent (macOS) =="
echo

[ -n "${STATUSPAGE_URL:-}" ] || STATUSPAGE_URL="$(ask 'URL статус-страницы (https://...)')"
[ -n "${ADMIN_TOKEN:-}" ] || ADMIN_TOKEN="$(asks 'ADMIN_TOKEN статус-страницы')"
DEFAULT_NAME="Mac-$(scutil --get ComputerName 2>/dev/null || hostname -s || echo "$(whoami)")"
[ -n "${PROBE_NAME:-}" ] || PROBE_NAME="$(ask 'Имя пробника' "${DEFAULT_NAME}")"
INTERVAL="${INTERVAL:-60}"
EXPECT_COUNTRY="${EXPECT_COUNTRY:-RU}"

[ -n "${STATUSPAGE_URL}" ] && [ -n "${ADMIN_TOKEN}" ] \
  || die "URL статус-страницы и ADMIN_TOKEN обязательны."

STATUSPAGE_URL="${STATUSPAGE_URL%/}"

c_g "→ регистрируем пробник на сервере…"
RESP="$(curl -fsSL -X POST \
  -H "X-Admin-Token: ${ADMIN_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"${PROBE_NAME}\",\"mode\":\"merge\"}" \
  "${STATUSPAGE_URL}/api/admin/probes")" \
  || die "регистрация не удалась. Проверь URL и ADMIN_TOKEN."
PROBE_ID="$(echo "${RESP}" | "${PYTHON}" -c 'import sys,json;print(json.load(sys.stdin)["probe_id"])')" \
  || die "не удалось распарсить ответ сервера: ${RESP}"
PROBE_TOKEN="$(echo "${RESP}" | "${PYTHON}" -c 'import sys,json;print(json.load(sys.stdin)["probe_token"])')"
echo "  probe_id: ${PROBE_ID}"

c_g "→ устанавливаем агент в ${APP_DIR}…"
mkdir -p "${APP_DIR}"
mkdir -p "$(dirname "${LOG_FILE}")"
mkdir -p "$(dirname "${PLIST}")"
SELF_DIR="$(cd "$(dirname "$0")" && pwd)"
MONITORVPN_RAW_URL="https://raw.githubusercontent.com/Mrvibecodic/xray-checker-statuspage/main/probes/monitorvpn"
if [ -f "${SELF_DIR}/agent.py" ]; then
  cp "${SELF_DIR}/agent.py" "${APP_DIR}/agent.py"
  echo "  agent.py скопирован из ${SELF_DIR}"
else
  curl -fsSL "${AGENT_RAW_URL}" -o "${APP_DIR}/agent.py" \
    || die "не удалось скачать agent.py с GitHub"
  echo "  agent.py скачан с GitHub"
fi
chmod 600 "${APP_DIR}/agent.py"

if [ -f "${SELF_DIR}/monitorvpn" ]; then
  cp "${SELF_DIR}/monitorvpn" "${APP_DIR}/monitorvpn"
else
  curl -fsSL "${MONITORVPN_RAW_URL}" -o "${APP_DIR}/monitorvpn" \
    || c_y "не удалось скачать monitorvpn с GitHub (необязательно)"
fi
chmod +x "${APP_DIR}/monitorvpn" 2>/dev/null || true

# Положим symlink в /usr/local/bin, если есть права. Иначе подскажем как.
if [ -w /usr/local/bin ] 2>/dev/null; then
  ln -sf "${APP_DIR}/monitorvpn" /usr/local/bin/monitorvpn
  echo "  команда monitorvpn доступна глобально (/usr/local/bin/monitorvpn)"
elif command -v sudo >/dev/null 2>&1; then
  if sudo -n true 2>/dev/null; then
    sudo ln -sf "${APP_DIR}/monitorvpn" /usr/local/bin/monitorvpn
    echo "  команда monitorvpn доступна глобально (через sudo)"
  else
    c_y "→ чтобы вызывать команду как 'monitorvpn', выполни один раз:"
    echo "    sudo ln -sf ${APP_DIR}/monitorvpn /usr/local/bin/monitorvpn"
    echo "  пока можно запускать через полный путь: ${APP_DIR}/monitorvpn status"
  fi
else
  c_y "→ symlink в PATH не создан. Запускай через полный путь:"
  echo "    ${APP_DIR}/monitorvpn status"
fi

c_g "→ регистрируем LaunchAgent ${LABEL}…"
cat > "${PLIST}" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>${LABEL}</string>
  <key>ProgramArguments</key>
  <array>
    <string>${PYTHON}</string>
    <string>${APP_DIR}/agent.py</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>ThrottleInterval</key><integer>30</integer>
  <key>StandardOutPath</key><string>${LOG_FILE}</string>
  <key>StandardErrorPath</key><string>${LOG_FILE}</string>
  <key>EnvironmentVariables</key>
  <dict>
    <key>STATUSPAGE_URL</key><string>${STATUSPAGE_URL}</string>
    <key>PROBE_TOKEN</key><string>${PROBE_TOKEN}</string>
    <key>INTERVAL</key><string>${INTERVAL}</string>
    <key>EXPECT_COUNTRY</key><string>${EXPECT_COUNTRY}</string>
  </dict>
</dict>
</plist>
EOF
chmod 600 "${PLIST}"

# Если агент уже стоял — выгружаем перед перезаливкой.
if launchctl print "gui/$(id -u)/${LABEL}" >/dev/null 2>&1; then
  launchctl bootout "gui/$(id -u)/${LABEL}" 2>/dev/null || true
elif launchctl list "${LABEL}" >/dev/null 2>&1; then
  launchctl unload "${PLIST}" 2>/dev/null || true
fi

# Современный путь — bootstrap; fallback на старый load.
if ! launchctl bootstrap "gui/$(id -u)" "${PLIST}" 2>/dev/null; then
  launchctl load "${PLIST}" || die "не удалось загрузить LaunchAgent."
fi

echo
c_g "✓ агент запущен. При следующих логинах будет стартовать сам."
echo "   Управление:  monitorvpn {start|stop|restart|status|logs|delete}"
echo "   Логи:        tail -f \"${LOG_FILE}\""
echo "   Снести:      monitorvpn delete   (или: bash \"$0\" --uninstall)"
echo
c_y "Подсказка: первая строка лога должна быть «start: interval=...» через ~5 сек."
