package bot

import "github.com/go-telegram/bot/models"

func ikb(text, data string) models.InlineKeyboardButton {
	return models.InlineKeyboardButton{Text: text, CallbackData: data}
}

// pairRows раскладывает плоский список кнопок по 2 в ряд.
func pairRows(btns []models.InlineKeyboardButton) [][]models.InlineKeyboardButton {
	var rows [][]models.InlineKeyboardButton
	for i := 0; i < len(btns); i += 2 {
		if i+1 < len(btns) {
			rows = append(rows, []models.InlineKeyboardButton{btns[i], btns[i+1]})
		} else {
			rows = append(rows, []models.InlineKeyboardButton{btns[i]})
		}
	}
	return rows
}

func mainMenuKB() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{ikb("🖥 Серверы", "m:servers"), ikb("🚨 Инциденты", "m:incidents")},
		{ikb("🛠 Обслуживание", "m:maint"), ikb("🚀 Веб-сервер", "m:web")},
		{ikb("⚙️ Ещё", "m:more")},
		{ikb("🔄 Обновить статус", "m:refresh")},
	}}
}

// moreKB — редко используемые разделы, чтобы не загромождать главную.
func moreKB() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{ikb("📈 Статистика", "m:stats"), ikb("👁 Видимость", "m:vis")},
		{ikb("🔌 Подписка", "m:sub"), ikb("⬆️ Обновление", "m:update")},
		{ikb("🎨 Вид страницы", "m:page"), ikb("⚙️ Настройки", "m:settings")},
		{ikb("🧹 База / очистка", "m:clean")},
		{ikb("📜 Журнал", "m:audit")},
		{ikb("◀ Меню", "m:home")},
	}}
}

func updateKB() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{ikb("🔍 Проверить", "m:upcheck"), ikb("⬆️ Обновить", "m:doupdate")},
		{ikb("🔁 Перезапуск", "m:restart"), ikb("◀ Ещё", "m:more")},
	}}
}

// moreBackKB — «назад» в раздел «Ещё» для его дочерних секций.
func moreBackKB() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{ikb("◀ Ещё", "m:more")},
	}}
}

func backKB() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{ikb("◀ Меню", "m:home")},
	}}
}

// sectionText — текст и клавиатура раздела по callback data. Всё навигируется
// правкой одного сообщения (без новых сообщений).
func (tb *Bot) sectionText(uid int64, data string) (string, *models.InlineKeyboardMarkup) {
	switch data {
	case "m:more":
		return "<b>⚙️ Ещё</b>\nСтатистика, видимость серверов на странице, подписка чекера, обновление, настройки, очистка базы и журнал.", moreKB()
	case "m:page":
		return tb.pageText(), tb.pageKB()
	case "m:clean":
		return tb.cleanText(), tb.cleanKB()
	case "m:nginx":
		return cmdNginx(tb.st, tb.cfg), &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
			{ikb("◀ Веб-сервер", "m:web")},
		}}
	case "m:status":
		return statusText(tb.st, tb.cfg), backKB()
	case "m:servers":
		return serversText(tb.st, tb.cfg), backKB()
	case "m:stats":
		return statsText(tb.st, tb.cfg), moreBackKB()
	case "m:incidents":
		return tb.incidentsText(), tb.incidentsKB()
	case "m:maint":
		return maintText(tb.st, tb.cfg), maintKB(tb.st, tb.cfg)
	case "m:vis":
		tb.setPage(uid, "vis_pg", 0)
		return tb.visText(), tb.visKB(uid)
	case "m:sub":
		return tb.subText(), tb.subKB()
	case "m:settings":
		return tb.settingsText(), tb.settingsKB()
	case "m:web":
		return tb.webText(), tb.webKB()
	case "m:audit":
		return cmdAudit(tb), moreBackKB()
	case "m:update":
		return tb.updateText(), updateKB()
	default: // m:home и неизвестное
		return tb.mainMenuText(), mainMenuKB()
	}
}
