package bot

import (
	"fmt"
	"strings"
	"time"

	"xray-status/internal/geo"
	"xray-status/internal/store"
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

// resolveServerNames находит «сырые» имена серверов (как в чекере), совпадающие
// по подстроке с запросом — по raw-имени или по отображаемому имени.
func resolveServerNames(st *store.Store, query string) []string {
	rows, err := st.CurrentRows()
	if err != nil {
		return nil
	}
	q := strings.ToLower(strings.TrimSpace(query))
	seen := map[string]bool{}
	var out []string
	for _, r := range rows {
		raw := r.Name
		disp := geo.DisplayName(raw, geo.DetectCountry(raw))
		if strings.Contains(strings.ToLower(raw), q) || strings.Contains(strings.ToLower(disp), q) {
			if !seen[raw] {
				seen[raw] = true
				out = append(out, raw)
			}
		}
	}
	return out
}

func cmdSub(tb *Bot, args []string, author int64) string {
	if !tb.canControl("subscription") {
		return "Управление подпиской выключено. Добавь <code>subscription</code> в CONTROL_CAPS."
	}
	if len(args) >= 1 && args[0] == "url" {
		if !tb.st.SecretsEnabled() {
			return "Нужен SECRET_KEY, чтобы хранить подписку зашифрованно."
		}
		url := strings.TrimSpace(strings.Join(args[1:], " "))
		if url == "" {
			return "Укажи URL: /sub url https://provider/sub?token=…"
		}
		if err := tb.st.SetSubscriptionURL(url); err != nil {
			return "Ошибка: " + err.Error()
		}
		_ = tb.st.AddAudit(author, "sub_url_set", "", "", "ok")
		return "✅ Подписка сохранена.\nУкажи чекеру: <code>SUBSCRIPTION_URL=http://statuspage:" +
			tb.cfg.InternalPort + "/sub</code>"
	}
	var b strings.Builder
	b.WriteString("<b>Подписка чекера</b>\n")
	if tb.st.HasSubscriptionURL() {
		b.WriteString("Состояние: ✅ настроена\n")
	} else {
		b.WriteString("Состояние: ❌ не задана — /sub url &lt;URL&gt;\n")
	}
	metas, _ := tb.st.ServersMeta()
	off := 0
	for _, m := range metas {
		if !m.Enabled {
			off++
		}
	}
	fmt.Fprintf(&b, "Выключено серверов: <b>%d</b>\n", off)
	fmt.Fprintf(&b, "Чекеру: <code>SUBSCRIPTION_URL=http://statuspage:%s/sub</code>\n\n", tb.cfg.InternalPort)
	b.WriteString("Команды: /sub url &lt;URL&gt; · /server &lt;имя&gt; on|off")
	return b.String()
}

func cmdServerToggle(tb *Bot, args []string, author int64) string {
	if len(args) < 2 {
		return "Использование: /server &lt;имя&gt; on|off  (off — скрыть со страницы, on — показать)"
	}
	action := args[len(args)-1]
	if action != "on" && action != "off" {
		return "Последний аргумент — on (показать) или off (скрыть)."
	}
	name := strings.Join(args[:len(args)-1], " ")
	hide := action == "off"
	targets := resolveServerNames(tb.st, name)
	if len(targets) == 0 {
		return "Не нашёл сервер: " + htmlEscape(name)
	}
	for _, t := range targets {
		_ = tb.st.SetHiddenName(t, hide)
	}
	act := "server_show"
	if hide {
		act = "server_hide"
	}
	_ = tb.st.AddAudit(author, act, name, strings.Join(targets, ","), "ok")
	if hide {
		return fmt.Sprintf("🙈 Скрыто со страницы: <b>%s</b> (%d)", htmlEscape(name), len(targets))
	}
	return fmt.Sprintf("👁 Снова на странице: <b>%s</b> (%d)", htmlEscape(name), len(targets))
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
