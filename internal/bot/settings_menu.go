package bot

import (
	"strconv"
	"strings"

	"github.com/go-telegram/bot/models"

	"xray-status/internal/config"
)

// settingsText — текст раздела настроек (значения вкл/пинг/сводка показаны на
// кнопках; здесь — домен и БД).
func tzShort(cfg config.Config) string {
	if cfg.TZ == "Europe/Moscow" || cfg.TZ == "" {
		return "МСК"
	}
	return cfg.TZ
}

func themeName(code string) string {
	switch code {
	case "light":
		return "светлая"
	case "claude":
		return "Claude"
	case "claude-dark":
		return "Claude Code"
	case "v2":
		return "тема 2.0"
	case "minimal":
		return "минималистичная"
	default:
		return "тёмная"
	}
}

func (tb *Bot) checkerIntervalMin() string {
	n := tb.cfg.PollInterval
	if v := tb.st.GetSetting("checker_interval", ""); v != "" {
		if x, err := strconv.Atoi(v); err == nil && x >= 10 {
			n = x
		}
	}
	return trimNum(float64(n) / 60.0)
}

func (tb *Bot) settingsText() string {
	return "<b>Настройки</b>\nУведомления. Вид страницы (заголовок, тема, фавикон) — в «🎨 Вид страницы»; домен — в «🚀 Веб-сервер».\n\n" +
		"🔄 Интервал проверок: <b>" + tb.checkerIntervalMin() + " мин</b> (из чекера)\n" +
		"   меняется в compose у xray-checker: PROXY_CHECK_INTERVAL\n" +
		"🗄 БД: <b>" + tb.st.Driver() + "</b>"
}

func trimNum(f float64) string {
	s := strconv.FormatFloat(f, 'f', 1, 64)
	if strings.HasSuffix(s, ".0") {
		return s[:len(s)-2]
	}
	return s
}

func (tb *Bot) settingsKB() *models.InlineKeyboardMarkup {
	st := tb.st
	alert := "выкл"
	if st.AlertOnDown() {
		alert = "вкл ✅"
	}
	ping := "выкл"
	if p := st.PingThreshold(); p > 0 {
		ping = strconv.Itoa(p) + " мс"
	}
	sum := st.DailySummaryTime()
	if sum == "" {
		sum = "выкл"
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{ikb("🔔 Алерт: "+alert, "set:alert"), ikb("📡 Пинг: "+ping, "set:ping")},
		{ikb("🗓 Сводка: "+sum, "set:sum")},
		{ikb("🔕 Тихие серверы", "set:mute")},
		{ikb("◀ Ещё", "m:more")},
	}}
}

func themeKB() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{ikb("🌞 Светлая", "set:theme:light"), ikb("🌙 Тёмная", "set:theme:dark")},
		{ikb("🟠 Claude", "set:theme:claude"), ikb("🟤 Claude Code", "set:theme:claude-dark")},
		{ikb("🆕 Тема 2.0", "set:theme:v2")},
		{ikb("🪶 Минималистичная", "set:theme:minimal")},
		{ikb("◀ Назад", "m:page")},
	}}
}

func pingKB() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{ikb("Выкл", "set:ping:0"), ikb("200 мс", "set:ping:200")},
		{ikb("300 мс", "set:ping:300"), ikb("500 мс", "set:ping:500")},
		{ikb("1000 мс", "set:ping:1000")},
		{ikb("◀ Назад", "set:home")},
	}}
}

func summaryKB() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{ikb("Выкл", "set:sum:off"), ikb("08:00", "set:sum:08:00")},
		{ikb("09:00", "set:sum:09:00"), ikb("12:00", "set:sum:12:00")},
		{ikb("18:00", "set:sum:18:00"), ikb("21:00", "set:sum:21:00")},
		{ikb("◀ Назад", "set:home")},
	}}
}

// handleSettingCallback обрабатывает нажатия "set:*", меняет настройку и
// возвращает новый текст+клавиатуру (правка того же сообщения). Для домена
// переводит чат в режим ожидания ввода (msgID — какое сообщение потом обновить).
func (tb *Bot) handleSettingCallback(uid int64, msgID int, data string) (string, *models.InlineKeyboardMarkup) {
	switch {
	case data == "set:home":
		tb.st.DelBotState(uid, "await")
		tb.st.DelBotState(uid, "await_msg")
		return tb.settingsText(), tb.settingsKB()
	case data == "set:alert":
		_ = tb.st.SetAlertOnDown(!tb.st.AlertOnDown())
		return tb.settingsText(), tb.settingsKB()
	case data == "set:mute":
		tb.setPage(uid, "mute_pg", 0)
		return tb.muteText(), tb.muteKB(uid)
	case data == "set:ping":
		return "📡 Порог высокого пинга:", pingKB()
	case len(data) > 9 && data[:9] == "set:ping:":
		n, _ := strconv.Atoi(data[9:])
		_ = tb.st.SetPingThreshold(n)
		return tb.settingsText(), tb.settingsKB()
	case data == "set:sum":
		return "🗓 Время ежедневной сводки (по " + tzShort(tb.cfg) + "):", summaryKB()
	case len(data) > 8 && data[:8] == "set:sum:":
		v := data[8:]
		if v == "off" {
			v = ""
		}
		_ = tb.st.SetDailySummaryTime(v)
		return tb.settingsText(), tb.settingsKB()
	case data == "set:theme":
		return "🎨 Тема публичной страницы:", themeKB()
	case strings.HasPrefix(data, "set:theme:"):
		v := data[len("set:theme:"):]
		switch v {
		case "light", "dark", "claude", "claude-dark", "v2", "minimal":
			_ = tb.st.SetSetting("theme", v)
			_ = tb.st.AddAudit(uid, "theme_set", "", v, "ok")
		}
		return tb.pageText(), tb.pageKB()
	case data == "set:title", data == "set:subtitle", data == "set:desc":
		key := map[string]string{"set:title": "page_title", "set:subtitle": "page_subtitle", "set:desc": "page_desc"}[data]
		prompt := map[string]string{
			"set:title":    "📝 Отправь новый заголовок страницы сообщением.",
			"set:subtitle": "🏷 Отправь новый подзаголовок сообщением.",
			"set:desc":     "🔖 Отправь мета-описание страницы (для SEO/предпросмотра ссылки).",
		}[data]
		_ = tb.st.SetBotState(uid, "await", key)
		_ = tb.st.SetBotState(uid, "await_msg", strconv.Itoa(msgID))
		return prompt + "\nТвоё сообщение удалится автоматически.", pageCancelKB()
	case data == "set:favicon":
		_ = tb.st.SetBotState(uid, "await", "favicon")
		_ = tb.st.SetBotState(uid, "await_msg", strconv.Itoa(msgID))
		return "🖼 Пришли картинку фавикона — PNG/SVG/ICO (лучше документом, чтобы без сжатия).", pageCancelKB()
	case data == "set:favreset":
		_ = tb.st.DelAsset("favicon")
		_ = tb.st.AddAudit(uid, "favicon_reset", "", "", "ok")
		return tb.pageText(), tb.pageKB()
	case data == "set:domain":
		_ = tb.st.SetBotState(uid, "await", "domain")
		_ = tb.st.SetBotState(uid, "await_msg", strconv.Itoa(msgID))
		return "🌐 Отправь домен сообщением (например <code>status.example.com</code>).\n" +
				"Твоё сообщение удалится автоматически.",
			&models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{ikb("✖ Отмена", "m:web")}}}
	default:
		return tb.settingsText(), tb.settingsKB()
	}
}
