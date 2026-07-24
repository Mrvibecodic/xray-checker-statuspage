package bot

import "github.com/go-telegram/bot/models"

func (tb *Bot) muteText() string {
	return "<b>🔕 Тихие серверы</b>\n" +
		"Жми по серверу — заглушить/включить уведомления по нему (падение, восстановление, высокий пинг).\n" +
		"Сервер остаётся в мониторинге и на странице — молчат только алерты.\n" +
		"🔔 — алерты идут · 🔕 — заглушён"
}

func (tb *Bot) muteKB(uid int64) *models.InlineKeyboardMarkup {
	var btns []models.InlineKeyboardButton
	muted, _ := tb.st.MutedSet()
	for _, name := range tb.groupNames() {
		mark := "🔔"
		if muted[name] {
			mark = "🔕"
		}
		btns = append(btns, ikb(mark+" "+name, "mute:"+nameTok(name)))
	}
	rows := paginateRows(btns, tb.getPage(uid, "mute_pg"), "mute:pg:")
	rows = append(rows, []models.InlineKeyboardButton{ikb("◀ Настройки", "set:home")})
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func (tb *Bot) handleMuteCallback(uid int64, data string) (string, *models.InlineKeyboardMarkup) {
	if len(data) > len("mute:pg:") && data[:len("mute:pg:")] == "mute:pg:" {
		tb.setPage(uid, "mute_pg", atoiSafe(data[len("mute:pg:"):]))
		return tb.muteText(), tb.muteKB(uid)
	}
	name := tb.resolveName(data[len("mute:"):])
	if name == "" {
		return tb.muteText(), tb.muteKB(uid)
	}
	muted, _ := tb.st.MutedSet()
	_ = tb.st.SetMutedName(name, !muted[name])
	act := "server_unmute"
	if !muted[name] {
		act = "server_mute"
	}
	_ = tb.st.AddAudit(uid, act, name, "", "ok")
	return tb.muteText(), tb.muteKB(uid)
}
