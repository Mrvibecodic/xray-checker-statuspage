package bot

import (
	"fmt"
	"strings"

	"xray-status/internal/config"
	"xray-status/internal/poller"
	"xray-status/internal/store"
	"xray-status/internal/summary"
)

// flag превращает ISO-код страны в эмодзи-флаг (regional indicators).
func flag(cc string) string {
	if len(cc) != 2 {
		return ""
	}
	cc = strings.ToUpper(cc)
	a := rune(0x1F1E6 + int(cc[0]-'A'))
	b := rune(0x1F1E6 + int(cc[1]-'A'))
	return string([]rune{a, b})
}

// auditRes переводит ошибку записи в поле result журнала аудита: "ok" при
// успехе, "err" при ошибке — чтобы журнал не фиксировал «ok» для несостоявшегося
// действия.
func auditRes(err error) string {
	if err != nil {
		return "err"
	}
	return "ok"
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func fmtPct(v any) string {
	if v == nil {
		return "—"
	}
	if f, ok := v.(float64); ok {
		return fmt.Sprintf("%.2f%%", f)
	}
	return "—"
}

// statusText — краткая сводка для /status.
func statusText(st *store.Store, cfg config.Config) string {
	p, err := summary.BuildSummary(st, cfg, true)
	if err != nil {
		return "Ошибка чтения статуса: " + err.Error()
	}
	tot := p["totals"].(map[string]any)
	online := tot["online"].(int)
	total := tot["total"].(int)
	var b strings.Builder
	fmt.Fprintf(&b, "<b>%s</b>\n", htmlEscape(cfg.Title))
	mark := "✅"
	if online < total {
		mark = "⚠️"
	}
	if online == 0 && total > 0 {
		mark = "🔴"
	}
	fmt.Fprintf(&b, "%s Онлайн: <b>%d/%d</b>\n", mark, online, total)
	fmt.Fprintf(&b, "📈 Аптайм (%dд): %s\n", cfg.Days, fmtPct(tot["uptime30"]))
	fmt.Fprintf(&b, "⏱ Средняя задержка: %d мс\n", tot["avgLatency"].(int))
	if lc, ok := p["lastCheck"].(string); ok && lc != "" {
		fmt.Fprintf(&b, "🕒 Последняя проверка: %s", htmlEscape(lc))
	}
	return b.String()
}

// serversText — список серверов для /servers.
func serversText(st *store.Store, cfg config.Config) string {
	p, err := summary.BuildSummary(st, cfg, true)
	if err != nil {
		return "Ошибка: " + err.Error()
	}
	servers := p["servers"].([]any)
	if len(servers) == 0 {
		return "Серверов пока нет (ждём первый опрос чекера)."
	}
	var b strings.Builder
	b.WriteString("<b>Серверы</b>\n")
	for _, s := range servers {
		m := s.(map[string]any)
		online := m["online"].(bool)
		mark := "🔴"
		if online {
			mark = "🟢"
		}
		name := htmlEscape(m["name"].(string))
		fl := flag(m["cc"].(string))
		lat := m["latencyMs"].(int)
		latStr := "—"
		if online && lat > 0 {
			latStr = fmt.Sprintf("%d мс", lat)
		}
		fmt.Fprintf(&b, "%s %s %s · %s · аптайм %s",
			mark, fl, name, latStr, fmtPct(m["uptime30"]))
		if hid, _ := m["hidden"].(bool); hid {
			b.WriteString(" · 🙈скрыт")
		}
		if abs, _ := m["absent"].(bool); abs {
			b.WriteString(" · 👻нет в чекере")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// statsText — статистика аптайма/простоя для /stats.
func statsText(st *store.Store, cfg config.Config) string {
	p, err := summary.BuildSummary(st, cfg, true)
	if err != nil {
		return "Ошибка: " + err.Error()
	}
	servers := p["servers"].([]any)
	tot := p["totals"].(map[string]any)
	var b strings.Builder
	fmt.Fprintf(&b, "<b>Статистика за %d дней</b>\n", cfg.Days)
	fmt.Fprintf(&b, "Общий аптайм: %s · простой: %d мин\n\n",
		fmtPct(tot["uptime30"]), tot["downMin30"].(int))
	for _, s := range servers {
		m := s.(map[string]any)
		fmt.Fprintf(&b, "%s %s — аптайм %s, простой %d мин\n",
			flag(m["cc"].(string)), htmlEscape(m["name"].(string)),
			fmtPct(m["uptime30"]), m["downMin30"].(int))
	}
	return b.String()
}

// eventText форматирует алерт поллера. Пустая строка — событие не шлём.
func eventText(e poller.Event) string {
	switch e.Type {
	case poller.EventServerDown:
		return "🔴 <b>Сервер упал:</b> " + htmlEscape(e.Name)
	case poller.EventServerUp:
		return "🟢 <b>Сервер восстановлен:</b> " + htmlEscape(e.Name)
	case poller.EventGlobalOutageConfirmed:
		return fmt.Sprintf("🛑 <b>Глобальный сбой подтверждён</b> (%d/%d офлайн). Записываю как реальный простой.", e.Offline, e.Total)
	case poller.EventGlobalOutageCleared:
		return "✅ <b>Глобальный сбой снят</b> — связь с серверами восстановлена."
	case poller.EventHighPing:
		return fmt.Sprintf("🐢 <b>Высокий пинг:</b> %s — %d мс", htmlEscape(e.Name), e.Latency)
	case poller.EventPingOK:
		return fmt.Sprintf("🚀 <b>Пинг в норме:</b> %s — %d мс", htmlEscape(e.Name), e.Latency)
	default:
		return "" // suspected не шлём, чтобы не спамить до подтверждения
	}
}

// dailySummaryText — ежедневная сводка: текущий статус + итог по суткам.
func dailySummaryText(st *store.Store, cfg config.Config) string {
	base := statusText(st, cfg)
	todayDown := 0
	if p, err := summary.BuildSummary(st, cfg, false); err == nil {
		for _, sv := range p["servers"].([]any) {
			m := sv.(map[string]any)
			days, _ := m["days"].([]any)
			if len(days) > 0 {
				if dm, ok := days[len(days)-1].(map[string]any)["downMin"].(int); ok {
					todayDown += dm
				}
			}
		}
	}
	out := "🗓 <b>Ежедневная сводка</b>\n" + base
	if todayDown == 0 {
		out += "\n\n✅ За сутки падений не было."
	} else {
		out += fmt.Sprintf("\n\n⚠️ За сутки суммарный простой: <b>%d мин</b>.", todayDown)
	}
	return out
}

// mainMenuText — заголовок панели + живой статус прямо в тексте (отдельная
// кнопка «Статус» на главной больше не нужна).
func (tb *Bot) mainMenuText() string {
	if rows, err := tb.st.CurrentRows(); err == nil && len(rows) == 0 {
		return "<b>🎛 Панель управления</b>\n\n" + tb.noDataHint()
	}
	return "<b>🎛 Панель управления</b>\n\n" + liveStatus(tb.st, tb.cfg)
}

// noDataHint — почему данных нет: подписка не задана, или ждём проверку чекера.
func (tb *Bot) noDataHint() string {
	if !tb.st.HasSubscriptionURL() && tb.cfg.SubscriptionURL == "" {
		return "❌ Подписка не задана.\n\nЗадай её сначала:\n• в боте: «⚙️ Ещё» → «🔌 Подписка»\n• или в docker-compose (SUBSCRIPTION_URL у statuspage) и перезапусти контейнер"
	}
	return "⏳ Данных пока нет — ждём первую проверку чекера.\nСразу: <code>docker compose restart xray-checker</code>"
}

// liveStatus — компактная сводка для панели.
func liveStatus(st *store.Store, cfg config.Config) string {
	p, err := summary.BuildSummary(st, cfg, true)
	if err != nil {
		return "Ошибка чтения статуса: " + err.Error()
	}
	tot := p["totals"].(map[string]any)
	online := tot["online"].(int)
	total := tot["total"].(int)
	maint, _ := tot["maintenance"].(int)
	mark := "✅"
	if online < total {
		mark = "⚠️"
	}
	if online == 0 && total > 0 {
		mark = "🔴"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s Онлайн: <b>%d/%d</b>", mark, online, total)
	if maint > 0 {
		fmt.Fprintf(&b, " · 🛠 %d на обслуживании", maint)
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "📈 Аптайм (%dд): %s\n", cfg.Days, fmtPct(tot["uptime30"]))
	fmt.Fprintf(&b, "⏱ Средняя задержка: %d мс\n", tot["avgLatency"].(int))
	if incs, ok := p["incidents"].([]any); ok && len(incs) > 0 {
		fmt.Fprintf(&b, "🚨 Активных инцидентов: <b>%d</b>\n", len(incs))
	}
	if lc, ok := p["lastCheck"].(string); ok && lc != "" {
		fmt.Fprintf(&b, "🕒 Последняя проверка: %s", htmlEscape(lc))
	}
	return b.String()
}
