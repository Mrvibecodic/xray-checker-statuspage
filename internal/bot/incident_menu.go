package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"

	"xray-status/internal/store"
)

func (tb *Bot) incidentsText() string {
	incs, _ := tb.st.ActiveIncidents()
	if len(incs) == 0 {
		return "<b>🚨 Инциденты</b>\nАктивных инцидентов нет. 🟢"
	}
	var b strings.Builder
	b.WriteString("<b>🚨 Активные инциденты</b>\n")
	for _, in := range incs {
		fmt.Fprintf(&b, "#%d [%s] <b>%s</b> — %s\n", in.ID, in.Severity, htmlEscape(in.Title), in.Status)
	}
	return b.String()
}

func (tb *Bot) incidentsKB() *models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton
	incs, _ := tb.st.ActiveIncidents()
	for _, in := range incs {
		title := in.Title
		if r := []rune(title); len(r) > 24 {
			title = string(r[:24]) + "…"
		}
		rows = append(rows, []models.InlineKeyboardButton{
			ikb("🛠 #"+strconv.FormatInt(in.ID, 10)+" "+title, "inc:open:"+strconv.FormatInt(in.ID, 10)),
		})
	}
	rows = append(rows, []models.InlineKeyboardButton{ikb("➕ Создать инцидент", "inc:new")})
	rows = append(rows, []models.InlineKeyboardButton{ikb("◀ Меню", "m:home")})
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func incSevKB() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{ikb("🟡 Незначительный", "inc:sev:minor")},
		{ikb("🟠 Серьёзный", "inc:sev:major")},
		{ikb("🔴 Критический", "inc:sev:critical")},
		{ikb("◀ Назад", "m:incidents")},
	}}
}

func (tb *Bot) incidentDetail(id int64) string {
	in, err := tb.st.GetIncident(id)
	if err != nil {
		return "Инцидент не найден."
	}
	loc := tzOf(tb.cfg)
	var b strings.Builder
	fmt.Fprintf(&b, "<b>Инцидент #%d</b> [%s]\n%s\nСтатус: <b>%s</b>\n",
		in.ID, in.Severity, htmlEscape(in.Title), in.Status)
	if len(in.Affected) > 0 {
		fmt.Fprintf(&b, "🎯 Затронуты: <b>%s</b>\n", htmlEscape(strings.Join(in.Affected, ", ")))
	}
	b.WriteString("\n<b>Лента:</b>\n")
	ups, _ := tb.st.IncidentUpdates(id)
	for _, u := range ups {
		ts := time.Unix(u.TS, 0).In(loc).Format("02.01 15:04")
		fmt.Fprintf(&b, "• %s · <b>%s</b> %s\n", ts, u.Status, htmlEscape(u.Body))
	}
	return b.String()
}

func incStatusKB(id int64) *models.InlineKeyboardMarkup {
	sid := strconv.FormatInt(id, 10)
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{ikb("🔎 Расследуем", "inc:setst:"+sid+":investigating")},
		{ikb("🎯 Причина найдена", "inc:setst:"+sid+":identified")},
		{ikb("👀 Наблюдаем", "inc:setst:"+sid+":monitoring")},
		{ikb("✅ Закрыть инцидент", "inc:resolve:"+sid)},
		{ikb("◀ К инцидентам", "m:incidents")},
	}}
}

// --- выбор затронутых серверов при создании инцидента ---

func splitNL(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func toggleName(list []string, name string) []string {
	out := make([]string, 0, len(list)+1)
	found := false
	for _, x := range list {
		if x == name {
			found = true
			continue
		}
		out = append(out, x)
	}
	if !found {
		out = append(out, name)
	}
	return out
}

func (tb *Bot) incAffText(uid int64) string {
	sel := splitNL(tb.st.GetBotState(uid, "inc_aff"))
	t := "🎯 Выбери затронутые серверы (можно несколько), потом «Создать»."
	if len(sel) > 0 {
		t += "\nВыбрано: <b>" + htmlEscape(strings.Join(sel, ", ")) + "</b>"
	}
	return t
}

func (tb *Bot) incAffKB(uid int64) *models.InlineKeyboardMarkup {
	sel := splitNL(tb.st.GetBotState(uid, "inc_aff"))
	var btns []models.InlineKeyboardButton
	seen := map[string]bool{}
	cur, _ := tb.st.CurrentRows()
	for _, r := range cur {
		if seen[r.Name] {
			continue
		}
		seen[r.Name] = true
		label := r.Name
		if contains(sel, r.Name) {
			label = "✅ " + label
		}
		btns = append(btns, ikb(label, "inc:aff:"+nameTok(r.Name)))
	}
	rows := paginateRows(btns, tb.getPage(uid, "aff_pg"), "inc:affpg:")
	done := "✅ Создать"
	if len(sel) > 0 {
		done = fmt.Sprintf("✅ Создать (%d)", len(sel))
	}
	rows = append(rows,
		[]models.InlineKeyboardButton{ikb(done, "inc:affdone"), ikb("Без серверов", "inc:affnone")},
		[]models.InlineKeyboardButton{ikb("✖ Отмена", "m:incidents")},
	)
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func (tb *Bot) createIncidentFromState(uid int64, withAff bool) (string, *models.InlineKeyboardMarkup) {
	sev := tb.st.GetBotState(uid, "inc_sev")
	title := tb.st.GetBotState(uid, "inc_title_txt")
	if sev == "" {
		sev = "minor"
	}
	if title == "" {
		return tb.incidentsText(), tb.incidentsKB()
	}
	var aff []string
	if withAff {
		aff = splitNL(tb.st.GetBotState(uid, "inc_aff"))
	}
	_, cerr := tb.st.CreateIncident(title, sev, aff, "Инцидент создан", uid, false)
	_ = tb.st.AddAudit(uid, "incident_new", title, strings.Join(aff, ","), auditRes(cerr))
	tb.st.DelBotState(uid, "inc_sev")
	tb.st.DelBotState(uid, "inc_title_txt")
	tb.st.DelBotState(uid, "inc_aff")
	return tb.incidentsText(), tb.incidentsKB()
}

// handleIncCallback — кнопочное управление инцидентами.
func (tb *Bot) handleIncCallback(uid int64, msgID int, data string) (string, *models.InlineKeyboardMarkup) {
	switch {
	case data == "inc:new":
		return "Уровень нового инцидента:", incSevKB()
	case strings.HasPrefix(data, "inc:sev:"):
		sev := data[len("inc:sev:"):]
		_ = tb.st.SetBotState(uid, "inc_sev", sev)
		_ = tb.st.SetBotState(uid, "await", "inc_title")
		_ = tb.st.SetBotState(uid, "await_msg", itoa(msgID))
		return "✏️ Отправь заголовок инцидента сообщением (удалится автоматически).",
			&models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{ikb("✖ Отмена", "m:incidents")}}}
	case strings.HasPrefix(data, "inc:affpg:"):
		tb.setPage(uid, "aff_pg", atoiSafe(data[len("inc:affpg:"):]))
		return tb.incAffText(uid), tb.incAffKB(uid)
	case strings.HasPrefix(data, "inc:aff:"):
		name := tb.resolveName(data[len("inc:aff:"):])
		if name == "" {
			return tb.incAffText(uid), tb.incAffKB(uid)
		}
		_ = tb.st.SetBotState(uid, "inc_aff", strings.Join(toggleName(splitNL(tb.st.GetBotState(uid, "inc_aff")), name), "\n"))
		return tb.incAffText(uid), tb.incAffKB(uid)
	case data == "inc:affdone":
		return tb.createIncidentFromState(uid, true)
	case data == "inc:affnone":
		return tb.createIncidentFromState(uid, false)
	case strings.HasPrefix(data, "inc:open:"):
		id, _ := strconv.ParseInt(data[len("inc:open:"):], 10, 64)
		return tb.incidentDetail(id), incStatusKB(id)
	case strings.HasPrefix(data, "inc:setst:"):
		parts := strings.SplitN(data, ":", 4)
		if len(parts) == 4 {
			// id должен быть числом, а статус — из канонического набора (кнопки
			// шлют только его); иначе не пишем мусор в ленту.
			if id, err := strconv.ParseInt(parts[2], 10, 64); err == nil && contains(store.IncidentStatuses, parts[3]) {
				_ = tb.st.AddIncidentUpdate(id, parts[3], "статус обновлён", uid)
				return tb.incidentDetail(id), incStatusKB(id)
			}
		}
		return tb.incidentsText(), tb.incidentsKB()
	case strings.HasPrefix(data, "inc:resolve:"):
		id, _ := strconv.ParseInt(data[len("inc:resolve:"):], 10, 64)
		_ = tb.st.AddIncidentUpdate(id, "resolved", "закрыт", uid)
		_ = tb.st.AddAudit(uid, "incident_resolve", "#"+data[len("inc:resolve:"):], "", "ok")
		return tb.incidentsText(), tb.incidentsKB()
	default:
		return tb.incidentsText(), tb.incidentsKB()
	}
}
