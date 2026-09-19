package bot

import (
	"strconv"
	"strings"

	"github.com/go-telegram/bot/models"

	"xray-status/internal/store"
	"xray-status/internal/summary"
)

// Раздел «✏️ Имена серверов»: имя сервера на публичной странице, независимое
// от ремарки в подписке. Ключ — то же имя, что у скрытия/мьюта (для
// балансира — имя группы); на странице алиас идёт как есть.

func (tb *Bot) renText() string {
	return "<b>✏️ Имена серверов</b>\n" +
		"Жми по серверу — задать имя, под которым он виден на странице.\n" +
		"Статус и статистика в боте показывают то же имя, что страница; меню, алерты " +
		"и подписка — исходное. Флаг берётся из нового имени, а если страны в нём нет — из исходного."
}

func (tb *Bot) renKB(uid int64) *models.InlineKeyboardMarkup {
	aliases, _ := tb.st.Aliases()
	var btns []models.InlineKeyboardButton
	for _, name := range tb.groupNames() {
		label := "✏️ " + name
		if a := aliases[name]; a != "" {
			label += " → " + a
		}
		btns = append(btns, ikb(label, "ren:"+nameTok(name)))
	}
	rows := paginateRows(btns, tb.getPage(uid, "ren_pg"), "ren:pg:")
	rows = append(rows, []models.InlineKeyboardButton{ikb("◀ Ещё", "m:more")})
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func (tb *Bot) handleRenCallback(uid int64, msgID int, data string) (string, *models.InlineKeyboardMarkup) {
	tb.st.DelBotState(uid, "ren_target")
	switch {
	case strings.HasPrefix(data, "ren:pg:"):
		tb.setPage(uid, "ren_pg", atoiSafe(data[len("ren:pg:"):]))
		return tb.renText(), tb.renKB(uid)
	case strings.HasPrefix(data, "ren:reset:"):
		if name := tb.resolveName(data[len("ren:reset:"):]); name != "" {
			_ = tb.st.AddAudit(uid, "server_rename", name, "", auditRes(tb.st.SetAlias(name, "")))
		}
		return tb.renText(), tb.renKB(uid)
	}
	tok := data[len("ren:"):]
	name := tb.resolveName(tok)
	if name == "" {
		return tb.renText(), tb.renKB(uid)
	}
	aliases, _ := tb.st.Aliases()
	shown, _ := summary.PageName(name, aliases)
	_ = tb.st.SetBotState(uid, "ren_target", name)
	_ = tb.st.SetBotState(uid, "await", "ren_name")
	_ = tb.st.SetBotState(uid, "await_msg", strconv.Itoa(msgID))

	txt := "✏️ Сервер: <b>" + htmlEscape(name) + "</b>\n" +
		"Сейчас на странице: <b>" + htmlEscape(shown) + "</b>\n\n" +
		"Отправь новое имя сообщением (до " + itoa(store.AliasMaxRunes) + " символов).\n" +
		"Твоё сообщение удалится автоматически."
	var row []models.InlineKeyboardButton
	if aliases[name] != "" {
		row = append(row, ikb("♻ Исходное имя", "ren:reset:"+tok))
	}
	row = append(row, ikb("✖ Отмена", "m:ren"))
	return txt, &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{row}}
}

// applyRename — приём имени из режима ожидания ren_name.
func (tb *Bot) applyRename(chatID int64, txt string) {
	name := tb.st.GetBotState(chatID, "ren_target")
	tb.st.DelBotState(chatID, "ren_target")
	if name == "" || strings.TrimSpace(txt) == "" {
		return
	}
	_ = tb.st.AddAudit(chatID, "server_rename", name, txt, auditRes(tb.st.SetAlias(name, txt)))
}
