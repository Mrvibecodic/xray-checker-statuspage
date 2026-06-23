package bot

import (
	"strings"

	"github.com/go-telegram/bot/models"
)

// Раздел «Вид страницы»: заголовок, подзаголовок, мета-описание, тема, фавикон.
// Всё применяется на публичной странице на лету (через настройки в БД).

func (tb *Bot) pageTitle() string    { return tb.st.GetSetting("title", tb.cfg.Title) }
func (tb *Bot) pageSubtitle() string { return tb.st.GetSetting("subtitle", tb.cfg.Subtitle) }
func (tb *Bot) pageDesc() string     { return tb.st.GetSetting("description", tb.cfg.Description) }

func (tb *Bot) pageText() string {
	desc := tb.pageDesc()
	if desc == "" {
		desc = "(пусто)"
	}
	fav := "встроенный логотип"
	if _, _, ok := tb.st.GetAsset("favicon"); ok {
		fav = "загружен из бота"
	}
	var b strings.Builder
	b.WriteString("<b>🎨 Вид страницы</b>\n")
	b.WriteString("📝 Заголовок: <b>" + htmlEscape(tb.pageTitle()) + "</b>\n")
	b.WriteString("🏷 Подзаголовок: <b>" + htmlEscape(tb.pageSubtitle()) + "</b>\n")
	b.WriteString("🔖 Мета-описание: <b>" + htmlEscape(desc) + "</b>\n")
	b.WriteString("🎨 Тема: <b>" + themeName(tb.st.GetSetting("theme", "dark")) + "</b>\n")
	b.WriteString("🖼 Фавикон: <b>" + fav + "</b>")
	return b.String()
}

func (tb *Bot) pageKB() *models.InlineKeyboardMarkup {
	rows := [][]models.InlineKeyboardButton{
		{ikb("📝 Заголовок", "set:title"), ikb("🏷 Подзаголовок", "set:subtitle")},
		{ikb("🔖 Мета-описание", "set:desc"), ikb("🎨 Тема", "set:theme")},
		{ikb("🖼 Загрузить фавикон", "set:favicon")},
	}
	if _, _, ok := tb.st.GetAsset("favicon"); ok {
		rows = append(rows, []models.InlineKeyboardButton{ikb("♻ Сбросить фавикон", "set:favreset")})
	}
	rows = append(rows, []models.InlineKeyboardButton{ikb("◀ Ещё", "m:more")})
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func pageCancelKB() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{ikb("✖ Отмена", "m:page")},
	}}
}
