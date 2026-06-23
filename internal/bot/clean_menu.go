package bot

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-telegram/bot/models"

	"xray-status/internal/summary"
)

// Раздел «База / очистка»: тумблеры авто-удаления устаревших записей и
// антидребезга глобальных сбоев + удаление серверов, пропавших из чекера.
// Раньше это жило тумблерами на публичной странице — перенесено в бот.

func (tb *Bot) autocleanOn() bool {
	def := "0"
	if tb.cfg.StaleAfterHours > 0 {
		def = "1"
	}
	return tb.st.GetSetting("autoclean", def) == "1"
}

func (tb *Bot) staleHours() int {
	n, err := strconv.Atoi(tb.st.GetSetting("stale_hours", strconv.Itoa(tb.cfg.StaleAfterHours)))
	if err != nil || n < 0 {
		return tb.cfg.StaleAfterHours
	}
	return n
}

func (tb *Bot) globalConfirmOn() bool {
	def := "0"
	if tb.cfg.GlobalOutageRatio <= 1.0 {
		def = "1"
	}
	return tb.st.GetSetting("skip_global", def) == "1"
}

func setBool(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func fmtHours(h int) string {
	switch {
	case h <= 0:
		return "выкл"
	case h%168 == 0:
		return strconv.Itoa(h/168) + " нед"
	case h%24 == 0:
		return strconv.Itoa(h/24) + " дн"
	default:
		return strconv.Itoa(h) + " ч"
	}
}

// absentServers — группы, которых нет в последнем опросе чекера.
func (tb *Bot) absentServers() []map[string]any {
	p, err := summary.BuildSummary(tb.st, tb.cfg, true)
	if err != nil {
		return nil
	}
	servers, _ := p["servers"].([]any)
	var out []map[string]any
	for _, it := range servers {
		e, ok := it.(map[string]any)
		if !ok {
			continue
		}
		if a, _ := e["absent"].(bool); a {
			out = append(out, e)
		}
	}
	return out
}

func (tb *Bot) cleanText() string {
	var b strings.Builder
	b.WriteString("<b>🧹 База / очистка</b>\n")
	if tb.autocleanOn() && tb.staleHours() > 0 {
		fmt.Fprintf(&b, "🧹 Авто-удаление устаревших: <b>вкл</b> (нет в чекере дольше %s)\n", fmtHours(tb.staleHours()))
	} else {
		b.WriteString("🧹 Авто-удаление устаревших: <b>выкл</b>\n")
	}
	if tb.globalConfirmOn() {
		b.WriteString("🌀 Антидребезг глобальных сбоёв: <b>вкл</b> (разовый сбой не пишем)\n")
	} else {
		b.WriteString("🌀 Антидребезг глобальных сбоёв: <b>выкл</b>\n")
	}
	n := len(tb.absentServers())
	fmt.Fprintf(&b, "🗑 Отсутствуют в чекере сейчас: <b>%d</b>", n)
	return b.String()
}

func (tb *Bot) cleanKB() *models.InlineKeyboardMarkup {
	auto := "🧹 Авто-удаление: выкл"
	if tb.autocleanOn() && tb.staleHours() > 0 {
		auto = "🧹 Авто-удаление: " + fmtHours(tb.staleHours())
	}
	glob := "🌀 Антидребезг: выкл"
	if tb.globalConfirmOn() {
		glob = "🌀 Антидребезг: вкл"
	}
	rows := pairRows([]models.InlineKeyboardButton{
		ikb(auto, "cl:auto"), ikb(glob, "cl:glob"),
	})
	if n := len(tb.absentServers()); n > 0 {
		rows = append(rows, []models.InlineKeyboardButton{ikb(fmt.Sprintf("🗑 Отсутствующие (%d)", n), "cl:absent")})
	}
	rows = append(rows, []models.InlineKeyboardButton{ikb("◀ Назад", "m:more")})
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func cleanHoursKB() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{ikb("Выкл", "cl:h:0"), ikb("24 ч", "cl:h:24")},
		{ikb("2 дн", "cl:h:48"), ikb("3 дн", "cl:h:72")},
		{ikb("7 дн", "cl:h:168")},
		{ikb("◀ Назад", "m:clean")},
	}}
}

func (tb *Bot) absentKB(uid int64) *models.InlineKeyboardMarkup {
	var rows []models.InlineKeyboardButton
	for _, e := range tb.absentServers() {
		sid, _ := e["sid"].(string)
		name, _ := e["name"].(string)
		if sid == "" {
			continue
		}
		rows = append(rows, ikb("🗑 "+name, "cl:del:"+sid))
	}
	out := paginateRows(rows, tb.getPage(uid, "absent_pg"), "cl:abspg:")
	out = append(out, []models.InlineKeyboardButton{ikb("◀ Назад", "m:clean")})
	return &models.InlineKeyboardMarkup{InlineKeyboard: out}
}

func (tb *Bot) absentName(sid string) string {
	for _, e := range tb.absentServers() {
		if s, _ := e["sid"].(string); s == sid {
			n, _ := e["name"].(string)
			return n
		}
	}
	return sid
}

func (tb *Bot) handleCleanCallback(uid int64, data string) (string, *models.InlineKeyboardMarkup) {
	switch {
	case data == "cl:auto":
		return "🧹 Удалять серверы, пропавшие из чекера дольше:", cleanHoursKB()
	case strings.HasPrefix(data, "cl:h:"):
		h, _ := strconv.Atoi(data[len("cl:h:"):])
		if h < 0 {
			h = 0
		}
		_ = tb.st.SetSetting("stale_hours", strconv.Itoa(h))
		_ = tb.st.SetSetting("autoclean", setBool(h > 0))
		_ = tb.st.AddAudit(uid, "autoclean_set", "", fmtHours(h), "ok")
		return tb.cleanText(), tb.cleanKB()
	case data == "cl:glob":
		nv := !tb.globalConfirmOn()
		_ = tb.st.SetSetting("skip_global", setBool(nv))
		_ = tb.st.AddAudit(uid, "global_confirm_set", "", setBool(nv), "ok")
		return tb.cleanText(), tb.cleanKB()
	case data == "cl:absent":
		if len(tb.absentServers()) == 0 {
			return tb.cleanText(), tb.cleanKB()
		}
		tb.setPage(uid, "absent_pg", 0)
		return "🗑 Серверы, которых нет в чекере. Нажми, чтобы удалить из базы вместе с историей:", tb.absentKB(uid)
	case strings.HasPrefix(data, "cl:abspg:"):
		tb.setPage(uid, "absent_pg", atoiSafe(data[len("cl:abspg:"):]))
		return "🗑 Серверы, которых нет в чекере. Нажми, чтобы удалить из базы вместе с историей:", tb.absentKB(uid)
	case strings.HasPrefix(data, "cl:del:"):
		sid := data[len("cl:del:"):]
		name := tb.absentName(sid)
		return "Удалить «" + htmlEscape(name) + "» и всю его историю?",
			&models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
				{ikb("🗑 Удалить", "cl:dok:"+sid), ikb("✖ Отмена", "cl:absent")},
			}}
	case strings.HasPrefix(data, "cl:dok:"):
		sid := data[len("cl:dok:"):]
		name := tb.absentName(sid)
		if _, err := tb.st.DeleteServer(sid); err != nil {
			return "Ошибка удаления: " + htmlEscape(err.Error()), tb.cleanKB()
		}
		_ = tb.st.AddAudit(uid, "server_delete", name, sid, "ok")
		return tb.cleanText(), tb.cleanKB()
	default:
		return tb.cleanText(), tb.cleanKB()
	}
}
