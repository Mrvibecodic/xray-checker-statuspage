package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"

	"xray-status/internal/config"
	"xray-status/internal/store"
)

func tzOf(cfg config.Config) *time.Location {
	loc, err := time.LoadLocation(cfg.TZ)
	if err != nil {
		return time.UTC
	}
	return loc
}

func maintText(st *store.Store, cfg config.Config) string {
	loc := tzOf(cfg)
	ms, _ := st.ActiveMaintenance(time.Now().Unix())
	if len(ms) == 0 {
		return "<b>Обслуживание</b>\nАктивных работ нет.\nЖми «Запланировать»."
	}
	var b strings.Builder
	b.WriteString("<b>Обслуживание (идут работы)</b>\n")
	for _, m := range ms {
		until := "бессрочно"
		if m.ToTS > 0 {
			until = time.Unix(m.ToTS, 0).In(loc).Format("15:04")
		}
		fmt.Fprintf(&b, "• %s — до %s\n", htmlEscape(m.Name), until)
	}
	return b.String()
}

func maintKB(st *store.Store, _ config.Config) *models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton
	ms, _ := st.ActiveMaintenance(time.Now().Unix())
	for _, m := range ms {
		rows = append(rows, []models.InlineKeyboardButton{
			ikb("✖ Завершить: "+m.Name, "mnt:end:"+strconv.FormatInt(m.ID, 10)),
		})
	}
	rows = append(rows, []models.InlineKeyboardButton{ikb("➕ Запланировать", "mnt:new")})
	rows = append(rows, []models.InlineKeyboardButton{ikb("◀ Меню", "m:home")})
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func (tb *Bot) maintServersKB(uid int64) *models.InlineKeyboardMarkup {
	st := tb.st
	var btns []models.InlineKeyboardButton
	seen := map[string]bool{}
	cur, _ := st.CurrentRows()
	for _, r := range cur {
		if seen[r.Name] {
			continue
		}
		seen[r.Name] = true
		btns = append(btns, ikb(r.Name, "mnt:srv:"+nameTok(r.Name)))
	}
	rows := paginateRows(btns, tb.getPage(uid, "mnt_srv_pg"), "mnt:srvpg:")
	rows = append(rows, []models.InlineKeyboardButton{ikb("◀ Назад", "m:maint")})
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func maintDurKB() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{ikb("30 мин", "mnt:dur:30"), ikb("1 ч", "mnt:dur:60")},
		{ikb("2 ч", "mnt:dur:120"), ikb("6 ч", "mnt:dur:360")},
		{ikb("12 ч", "mnt:dur:720"), ikb("24 ч", "mnt:dur:1440")},
		{ikb("◀ Назад", "mnt:new")},
	}}
}

// handleMaintCallback — кнопочный поток планирования/завершения обслуживания.
func (tb *Bot) handleMaintCallback(uid int64, data string) (string, *models.InlineKeyboardMarkup) {
	now := time.Now().Unix()
	switch {
	case data == "mnt:new":
		tb.setPage(uid, "mnt_srv_pg", 0)
		return "Выбери сервер для работ:", tb.maintServersKB(uid)
	case strings.HasPrefix(data, "mnt:srvpg:"):
		tb.setPage(uid, "mnt_srv_pg", atoiSafe(data[len("mnt:srvpg:"):]))
		return "Выбери сервер для работ:", tb.maintServersKB(uid)
	case strings.HasPrefix(data, "mnt:srv:"):
		name := tb.resolveName(data[len("mnt:srv:"):])
		if name == "" {
			return "Выбери сервер для работ:", tb.maintServersKB(uid)
		}
		_ = tb.st.SetBotState(uid, "mnt_srv", name)
		return "Сервер: <b>" + htmlEscape(name) + "</b>\nНа сколько включить обслуживание?", maintDurKB()
	case strings.HasPrefix(data, "mnt:dur:"):
		mins, _ := strconv.Atoi(data[len("mnt:dur:"):])
		name := tb.st.GetBotState(uid, "mnt_srv")
		if name == "" || mins <= 0 {
			return maintText(tb.st, tb.cfg), maintKB(tb.st, tb.cfg)
		}
		_, _ = tb.st.AddMaintenance(name, now, now+int64(mins)*60, "", uid)
		_ = tb.st.DelBotState(uid, "mnt_srv")
		_ = tb.st.AddAudit(uid, "maint_add", name, strconv.Itoa(mins)+"м", "ok")
		return maintText(tb.st, tb.cfg), maintKB(tb.st, tb.cfg)
	case strings.HasPrefix(data, "mnt:end:"):
		id, _ := strconv.ParseInt(data[len("mnt:end:"):], 10, 64)
		_ = tb.st.EndMaintenance(id)
		_ = tb.st.AddAudit(uid, "maint_end", "#"+data[len("mnt:end:"):], "", "ok")
		return maintText(tb.st, tb.cfg), maintKB(tb.st, tb.cfg)
	default:
		return maintText(tb.st, tb.cfg), maintKB(tb.st, tb.cfg)
	}
}
