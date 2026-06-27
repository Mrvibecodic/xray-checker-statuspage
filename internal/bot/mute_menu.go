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
	cur, _ := tb.st.CurrentRows()
	muted, _ := tb.st.MutedSet()
	seen := map[string]bool{}
	for _, r := range cur {
		if seen[r.Name] {
			continue
		}
		seen[r.Name] = true
		mark := "🔔"
		if muted[r.Name] {
			mark = "🔕"
		}
		cb := "mute:" + r.Name
		if len(cb) <= 64 {
			btns = append(btns, ikb(mark+" "+r.Name, cb))
		}
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
	name := data[len("mute:"):]
	muted, _ := tb.st.MutedSet()
	_ = tb.st.SetMutedName(name, !muted[name])
	act := "server_unmute"
	if !muted[name] {
		act = "server_mute"
	}
	_ = tb.st.AddAudit(uid, act, name, "", "ok")
	return tb.muteText(), tb.muteKB(uid)
}
