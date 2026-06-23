package bot

import "github.com/go-telegram/bot/models"

const listPerPage = 10 // элементов на страницу (5 рядов по 2)

// paginateRows раскладывает плоский список кнопок по страницам (по 2 в ряд) и
// добавляет навигацию ◀ N/M ▶. navPrefix — префикс callback для перехода на
// страницу (к нему дописывается номер: navPrefix+"2").
func paginateRows(items []models.InlineKeyboardButton, page int, navPrefix string) [][]models.InlineKeyboardButton {
	per := listPerPage
	total := len(items)
	pages := (total + per - 1) / per
	if pages < 1 {
		pages = 1
	}
	if page < 0 {
		page = 0
	}
	if page >= pages {
		page = pages - 1
	}
	start := page * per
	end := start + per
	if end > total {
		end = total
	}
	rows := pairRows(items[start:end])
	if pages > 1 {
		var nav []models.InlineKeyboardButton
		if page > 0 {
			nav = append(nav, ikb("◀", navPrefix+itoa(page-1)))
		}
		nav = append(nav, ikb(itoa(page+1)+"/"+itoa(pages), "noop"))
		if page < pages-1 {
			nav = append(nav, ikb("▶", navPrefix+itoa(page+1)))
		}
		rows = append(rows, nav)
	}
	return rows
}

func atoiSafe(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// pageState читает/пишет текущую страницу списка в bot_state.
func (tb *Bot) getPage(uid int64, key string) int {
	return atoiSafe(tb.st.GetBotState(uid, key))
}
func (tb *Bot) setPage(uid int64, key string, page int) {
	_ = tb.st.SetBotState(uid, key, itoa(page))
}
