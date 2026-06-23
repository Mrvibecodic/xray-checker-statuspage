package bot

import "github.com/go-telegram/bot/models"

func (tb *Bot) subText() string {
	s := "<b>🔌 Подписка чекера</b>\n"
	switch {
	case tb.st.HasSubscriptionURL():
		s += "Состояние: 🟢 задана из бота\nЧекер тестирует именно её. Можешь заменить."
	case tb.cfg.SubscriptionURL != "":
		s += "Состояние: 🟢 активна (из compose)\nМожешь переопределить её прямо здесь."
	default:
		s += "Состояние: 🔴 не задана\nЗадай URL подписки — бот будет отдавать её чекеру."
	}
	return s
}

func (tb *Bot) subKB() *models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton
	if tb.canControl("subscription") {
		rows = append(rows, []models.InlineKeyboardButton{ikb("✏️ Задать URL подписки", "sub:url")})
	}
	rows = append(rows, []models.InlineKeyboardButton{ikb("◀ Ещё", "m:more")})
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func (tb *Bot) handleSubCallback(uid int64, msgID int, data string) (string, *models.InlineKeyboardMarkup) {
	switch data {
	case "sub:url":
		if !tb.canControl("subscription") {
			return tb.subText(), tb.subKB()
		}
		if !tb.st.SecretsEnabled() {
			return "Нужен SECRET_KEY для хранения подписки.", tb.subKB()
		}
		_ = tb.st.SetBotState(uid, "await", "sub_url")
		_ = tb.st.SetBotState(uid, "await_msg", itoa(msgID))
		return "✏️ Отправь URL подписки сообщением (удалится автоматически).",
			&models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{ikb("✖ Отмена", "m:sub")}}}
	default:
		return tb.subText(), tb.subKB()
	}
}
