package bot

import (
	"context"
	"html"
	"strconv"
	"strings"

	"github.com/go-telegram/bot/models"
)

func (tb *Bot) subText() string {
	urls := tb.st.SubscriptionURLs()
	s := "<b>🔌 Подписки чекера</b>\n"
	if len(urls) == 0 {
		if tb.cfg.SubscriptionURL != "" {
			s += "Состояние: 🟢 активна из compose\nМожешь добавить свои подписки здесь — они будут отданы чекеру одним списком."
		} else {
			s += "Состояние: 🔴 не заданы\nДобавь одну или несколько подписок — бот отдаст их чекеру одним списком."
		}
		return s + tb.subHint()
	}
	s += "Чекер тестирует все как один список. Сейчас " + itoa(len(urls)) + ":\n"
	for i, u := range urls {
		s += itoa(i+1) + ". " + maskURL(u) + "\n"
	}
	return s + tb.subHint()
}

// subHint — как подключить чекер и чтобы изменения подхватывались без рестарта.
func (tb *Bot) subHint() string {
	return "\n<b>Чекеру:</b> <code>SUBSCRIPTION_URL=http://localhost:" + tb.cfg.InternalPort + "/sub</code>\n" +
		"Изменения чекер подхватит сам по своему интервалу обновления подписки — рестарт не нужен."
}

// maskURL прячет токен/путь подписки, оставляя только хост для опознания —
// чтобы креды не светились в чате.
func maskURL(u string) string {
	host := u
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	cut := len(host)
	for _, sep := range []string{"/", "?", "#"} {
		if j := strings.Index(host, sep); j >= 0 && j < cut {
			cut = j
		}
	}
	h := strings.TrimSpace(host[:cut])
	if h == "" {
		h = "—"
	}
	return html.EscapeString(h) + "/…"
}

func (tb *Bot) subKB() *models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton
	if tb.canControl("subscription") {
		for i, u := range tb.st.SubscriptionURLs() {
			rows = append(rows, []models.InlineKeyboardButton{
				ikb("🗑 "+itoa(i+1)+" · "+maskHost(u), "sub:del:"+itoa(i)),
			})
		}
		rows = append(rows, []models.InlineKeyboardButton{ikb("➕ Добавить подписку", "sub:add")})
	}
	rows = append(rows, []models.InlineKeyboardButton{ikb("🩺 Диагностика", "sub:diag")})
	rows = append(rows, []models.InlineKeyboardButton{ikb("◀ Меню", "m:home")})
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// maskHost — короткий хост для подписи кнопки удаления (без HTML-экранирования:
// текст кнопки рендерится как plain).
func maskHost(u string) string {
	host := u
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	cut := len(host)
	for _, sep := range []string{"/", "?", "#"} {
		if j := strings.Index(host, sep); j >= 0 && j < cut {
			cut = j
		}
	}
	h := strings.TrimSpace(host[:cut])
	if h == "" {
		return "подписка"
	}
	return h
}

func (tb *Bot) handleSubCallback(uid int64, msgID int, data string) (string, *models.InlineKeyboardMarkup) {
	switch {
	case data == "sub:add":
		if !tb.canControl("subscription") {
			return tb.subText(), tb.subKB()
		}
		if !tb.st.SecretsEnabled() {
			return "Нужен SECRET_KEY для хранения подписок.", tb.subKB()
		}
		_ = tb.st.SetBotState(uid, "await", "sub_add")
		_ = tb.st.SetBotState(uid, "await_msg", itoa(msgID))
		return "➕ Пришли ссылки на подписки — через запятую, по строке на ссылку или каждую отдельным сообщением. Текущие не затираются.\n\nКогда закончишь — нажми «Готово».",
			&models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
				{ikb("✅ Готово", "m:sub")},
			}}
	case strings.HasPrefix(data, "sub:del:"):
		if !tb.canControl("subscription") {
			return tb.subText(), tb.subKB()
		}
		i, err := strconv.Atoi(strings.TrimPrefix(data, "sub:del:"))
		if err == nil {
			if removed, derr := tb.st.RemoveSubscriptionURLAt(i); derr == nil && removed != "" {
				_ = tb.st.AddAudit(uid, "sub_removed", maskHost(removed), "", "ok")
				go func() { tb.refreshNow(context.Background()) }()
			}
		}
		return tb.subText(), tb.subKB()
	default:
		return tb.subText(), tb.subKB()
	}
}
