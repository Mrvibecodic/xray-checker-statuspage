package bot

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"xray-status/internal/store"
)

var hhmmRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

func onOff(b bool) string {
	if b {
		return "вкл"
	}
	return "выкл"
}

// cmdSettings показывает текущие настройки.
func cmdSettings(st *store.Store) string {
	ping := st.PingThreshold()
	pingStr := "выкл"
	if ping > 0 {
		pingStr = fmt.Sprintf("%d мс", ping)
	}
	sum := st.DailySummaryTime()
	if sum == "" {
		sum = "выкл"
	}
	dom := st.PublicDomain()
	if dom == "" {
		dom = "(не задан)"
	}
	var b strings.Builder
	b.WriteString("<b>Настройки</b>\n")
	fmt.Fprintf(&b, "🔔 Алерт при падении: <b>%s</b>\n", onOff(st.AlertOnDown()))
	fmt.Fprintf(&b, "📡 Порог высокого пинга: <b>%s</b>\n", pingStr)
	fmt.Fprintf(&b, "🗓 Ежедневная сводка: <b>%s</b>\n", sum)
	fmt.Fprintf(&b, "🌐 Домен: <b>%s</b>\n", htmlEscape(dom))
	fmt.Fprintf(&b, "🗄 БД: <b>%s</b>\n\n", st.Driver())
	b.WriteString("Изменить: /set alert_down on|off · /set ping &lt;мс|off&gt; · " +
		"/set summary &lt;HH:MM|off&gt; · /set domain &lt;домен&gt;")
	return b.String()
}

// cmdSet меняет одну настройку.
func cmdSet(st *store.Store, args []string) string {
	if len(args) < 2 {
		return "Использование: /set &lt;ключ&gt; &lt;значение&gt;\n" +
			"ключи: alert_down (on|off), ping (мс|off), summary (HH:MM|off), domain (домен)"
	}
	key, val := args[0], strings.Join(args[1:], " ")
	switch key {
	case "alert_down", "alert":
		on := val == "on" || val == "вкл" || val == "1"
		if val != "on" && val != "off" && val != "вкл" && val != "выкл" && val != "1" && val != "0" {
			return "Значение: on или off"
		}
		if err := st.SetAlertOnDown(on); err != nil {
			return "Ошибка: " + err.Error()
		}
		return "🔔 Алерт при падении: <b>" + onOff(on) + "</b>"
	case "ping":
		if val == "off" || val == "0" {
			_ = st.SetPingThreshold(0)
			return "📡 Порог пинга выключен"
		}
		ms, err := strconv.Atoi(val)
		if err != nil || ms <= 0 {
			return "Значение: число мс (>0) или off"
		}
		if err := st.SetPingThreshold(ms); err != nil {
			return "Ошибка: " + err.Error()
		}
		return fmt.Sprintf("📡 Порог высокого пинга: <b>%d мс</b>", ms)
	case "summary":
		if val == "off" {
			_ = st.SetDailySummaryTime("")
			return "🗓 Ежедневная сводка выключена"
		}
		if !hhmmRe.MatchString(val) {
			return "Время в формате HH:MM (например 09:30) или off"
		}
		if err := st.SetDailySummaryTime(val); err != nil {
			return "Ошибка: " + err.Error()
		}
		return "🗓 Ежедневная сводка в <b>" + val + "</b>"
	case "domain":
		d := strings.TrimSpace(strings.ToLower(val))
		d = strings.TrimPrefix(d, "https://")
		d = strings.TrimPrefix(d, "http://")
		d = strings.TrimSuffix(d, "/")
		if d == "" {
			return "Укажи домен, например: /set domain status.example.com"
		}
		if err := st.SetPublicDomain(d); err != nil {
			return "Ошибка: " + err.Error()
		}
		return "🌐 Домен сохранён: <b>" + htmlEscape(d) + "</b>\nДальше: «🚀 Веб-сервер» — встроенный HTTPS, либо «🔧 Конфиг nginx» для реверса над ботом."
	default:
		return "Неизвестный ключ. Доступно: alert_down, ping, summary, domain"
	}
}
