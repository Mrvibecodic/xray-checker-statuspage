package bot

import "github.com/go-telegram/bot/models"

func (tb *Bot) visText() string {
	return "<b>👁 Видимость серверов</b>\n" +
		"Жми по серверу — скрыть/показать на публичной странице.\n" +
		"🟢 — показан · 🙈 — скрыт"
}

func (tb *Bot) visKB(uid int64) *models.InlineKeyboardMarkup {
	var btns []models.InlineKeyboardButton
	cur, _ := tb.st.CurrentRows()
	hidden, _ := tb.st.HiddenSet()
	seen := map[string]bool{}
	for _, r := range cur {
		if seen[r.Name] {
			continue
		}
		seen[r.Name] = true
		mark := "🟢"
		if hidden[r.Name] {
			mark = "🙈"
		}
		cb := "vis:" + r.Name
		if len(cb) <= 64 {
			btns = append(btns, ikb(mark+" "+r.Name, cb))
		}
	}
	rows := paginateRows(btns, tb.getPage(uid, "vis_pg"), "vis:pg:")
	rows = append(rows, []models.InlineKeyboardButton{ikb("◀ Ещё", "m:more")})
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func (tb *Bot) handleVisCallback(uid int64, data string) (string, *models.InlineKeyboardMarkup) {
	if len(data) > len("vis:pg:") && data[:len("vis:pg:")] == "vis:pg:" {
		tb.setPage(uid, "vis_pg", atoiSafe(data[len("vis:pg:"):]))
		return tb.visText(), tb.visKB(uid)
	}
	name := data[len("vis:"):]
	hidden, _ := tb.st.HiddenSet()
	_ = tb.st.SetHiddenName(name, !hidden[name])
	act := "server_show"
	if !hidden[name] {
		act = "server_hide"
	}
	_ = tb.st.AddAudit(uid, act, name, "", "ok")
	return tb.visText(), tb.visKB(uid)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
