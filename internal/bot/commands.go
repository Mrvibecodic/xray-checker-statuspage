package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"xray-status/internal/store"
)

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

func parseID(s string) (int64, bool) {
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil
}

// cmdIncident: new|update|resolve. args — токены после /incident.
func cmdIncident(st *store.Store, args []string, author int64) string {
	if len(args) == 0 {
		return "Использование:\n" +
			"/incident new &lt;minor|major|critical&gt; &lt;заголовок&gt;\n" +
			"/incident update &lt;id&gt; &lt;investigating|identified|monitoring&gt; &lt;текст&gt;\n" +
			"/incident resolve &lt;id&gt; &lt;текст&gt;"
	}
	switch args[0] {
	case "new":
		if len(args) < 3 {
			return "Нужно: /incident new &lt;severity&gt; &lt;заголовок&gt;"
		}
		sev := args[1]
		if !contains(store.IncidentSeverities, sev) {
			return "severity должно быть: minor, major или critical"
		}
		title := strings.Join(args[2:], " ")
		id, err := st.CreateIncident(title, sev, nil, "Инцидент создан", author, false)
		if err != nil {
			return "Ошибка: " + err.Error()
		}
		return fmt.Sprintf("🆕 Инцидент <b>#%d</b> создан (%s): %s", id, sev, htmlEscape(title))
	case "update":
		if len(args) < 4 {
			return "Нужно: /incident update &lt;id&gt; &lt;status&gt; &lt;текст&gt;"
		}
		id, ok := parseID(args[1])
		if !ok {
			return "id должен быть числом"
		}
		status := args[2]
		if !contains(store.IncidentStatuses, status) || status == "resolved" {
			return "status: investigating, identified или monitoring (для закрытия — /incident resolve)"
		}
		body := strings.Join(args[3:], " ")
		if err := st.AddIncidentUpdate(id, status, body, author); err != nil {
			return "Ошибка: " + err.Error()
		}
		return fmt.Sprintf("✏️ Инцидент #%d → <b>%s</b>", id, status)
	case "resolve":
		if len(args) < 2 {
			return "Нужно: /incident resolve &lt;id&gt; [текст]"
		}
		id, ok := parseID(args[1])
		if !ok {
			return "id должен быть числом"
		}
		body := "Инцидент закрыт"
		if len(args) > 2 {
			body = strings.Join(args[2:], " ")
		}
		if err := st.AddIncidentUpdate(id, "resolved", body, author); err != nil {
			return "Ошибка: " + err.Error()
		}
		return fmt.Sprintf("✅ Инцидент #%d закрыт", id)
	default:
		return "Неизвестное действие. /incident — покажет помощь."
	}
}

// cmdIncidents — список активных инцидентов.
func cmdIncidents(st *store.Store) string {
	incs, err := st.ActiveIncidents()
	if err != nil {
		return "Ошибка: " + err.Error()
	}
	if len(incs) == 0 {
		return "Активных инцидентов нет. 🟢"
	}
	var b strings.Builder
	b.WriteString("<b>Активные инциденты</b>\n")
	for _, in := range incs {
		fmt.Fprintf(&b, "#%d [%s] <b>%s</b> — %s\n", in.ID, in.Severity, htmlEscape(in.Title), in.Status)
	}
	return b.String()
}

// cmdMaintenance: без args — список; off &lt;имя&gt;; иначе &lt;минуты&gt; &lt;имя&gt; [| причина].
func cmdMaintenance(st *store.Store, args []string, author int64) string {
	if len(args) == 0 {
		ms, err := st.ActiveMaintenance(time.Now().Unix())
		if err != nil {
			return "Ошибка: " + err.Error()
		}
		if len(ms) == 0 {
			return "Активных работ нет.\nЗапланировать: /maintenance &lt;минуты&gt; &lt;сервер&gt; [| причина]"
		}
		var b strings.Builder
		b.WriteString("<b>Идут работы</b>\n")
		for _, m := range ms {
			until := "бессрочно"
			if m.ToTS > 0 {
				until = time.Unix(m.ToTS, 0).Format("15:04")
			}
			fmt.Fprintf(&b, "• %s — до %s%s\n", htmlEscape(m.Name), until, reasonSuffix(m.Reason))
		}
		return b.String()
	}
	if args[0] == "off" || args[0] == "end" {
		if len(args) < 2 {
			return "Нужно: /maintenance off &lt;сервер&gt;"
		}
		name := strings.Join(args[1:], " ")
		n, err := st.EndMaintenanceByName(name)
		if err != nil {
			return "Ошибка: " + err.Error()
		}
		if n == 0 {
			return "Не нашёл активных работ для: " + htmlEscape(name)
		}
		return "✅ Работы для " + htmlEscape(name) + " завершены."
	}
	mins, ok := parseID(args[0])
	if !ok || mins <= 0 {
		return "Первый аргумент — минуты (число). Пример: /maintenance 60 DE Frankfurt | апгрейд"
	}
	rest := strings.Join(args[1:], " ")
	server, reason := rest, ""
	if i := strings.IndexByte(rest, '|'); i >= 0 {
		server = strings.TrimSpace(rest[:i])
		reason = strings.TrimSpace(rest[i+1:])
	}
	if server == "" {
		return "Не указан сервер. Пример: /maintenance 60 DE Frankfurt | апгрейд"
	}
	now := time.Now().Unix()
	id, err := st.AddMaintenance(server, now, now+mins*60, reason, author)
	if err != nil {
		return "Ошибка: " + err.Error()
	}
	return fmt.Sprintf("🛠 Работы #%d для <b>%s</b> на %d мин.", id, htmlEscape(server), mins)
}

func reasonSuffix(r string) string {
	if strings.TrimSpace(r) == "" {
		return ""
	}
	return " (" + htmlEscape(r) + ")"
}
