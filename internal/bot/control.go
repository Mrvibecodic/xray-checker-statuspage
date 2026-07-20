package bot

import (
	"fmt"
	"strings"
	"time"
)

// canControl — разрешён ли слой управления (CONTROL_CAPS).
func (tb *Bot) canControl(capName string) bool {
	if len(tb.cfg.ControlCaps) == 0 {
		return true // по умолчанию управление включено (бот и так только для админов)
	}
	for _, c := range tb.cfg.ControlCaps {
		if c == capName {
			return true
		}
	}
	return false
}

func cmdAudit(tb *Bot) string {
	rows, err := tb.st.RecentAudit(15)
	if err != nil {
		return "Ошибка: " + err.Error()
	}
	if len(rows) == 0 {
		return "Журнал действий пуст."
	}
	loc, e := time.LoadLocation(tb.cfg.TZ)
	if e != nil {
		loc = time.UTC
	}
	var b strings.Builder
	b.WriteString("<b>Журнал действий</b>\n")
	for _, a := range rows {
		ts := time.Unix(a.TS, 0).In(loc).Format("02.01 15:04")
		tgt := ""
		if a.Target != "" {
			tgt = " · " + htmlEscape(a.Target)
		}
		fmt.Fprintf(&b, "%s · %d · <b>%s</b>%s\n", ts, a.Actor, htmlEscape(a.Action), tgt)
	}
	return b.String()
}
