package bot

import (
	"fmt"
	"strconv"
	"strings"

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
