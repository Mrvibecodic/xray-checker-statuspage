package poller

import "xray-status/internal/checker"

type EventType string

const (
	EventServerDown            EventType = "server_down"
	EventServerUp              EventType = "server_up"
	EventGlobalOutageSuspected EventType = "global_suspected"
	EventGlobalOutageConfirmed EventType = "global_confirmed"
	EventGlobalOutageCleared   EventType = "global_cleared"
	EventHighPing              EventType = "high_ping"
	EventPingOK                EventType = "ping_ok"
)

// Event — событие поллера для нотификаций (ПЛАН §7). Name — «сырое» имя группы;
// форматирование (флаг, display name) делает потребитель (бот).
type Event struct {
	Type    EventType
	Name    string
	Online  bool
	Offline int
	Total   int
	Latency int
}

// detectTransitions сравнивает свежий опрос с состоянием в current и возвращает
// переходы up/down на уровне группы (хост = группа одноимённых sid; группа
// «онлайн», если онлайн хотя бы один член). На первом цикле (нет prev) молчит.
func (p *Poller) detectTransitions(proxies []checker.Proxy, _ int64) []Event {
	// prev: онлайн-состояние групп по данным current.
	prevRows, err := p.st.CurrentRows()
	if err != nil {
		return nil
	}
	prev := map[string]bool{}     // name -> online
	prevSeen := map[string]bool{} // name присутствовал
	for _, r := range prevRows {
		prevSeen[r.Name] = true
		if r.Online == 1 {
			prev[r.Name] = true
		}
	}

	// now: онлайн-состояние групп из свежего опроса.
	now := map[string]bool{}
	seen := map[string]bool{}
	order := []string{}
	for _, px := range proxies {
		if px.StableID == "" {
			continue
		}
		name := px.Name
		if name == "" {
			name = px.StableID
		}
		if !seen[name] {
			seen[name] = true
			order = append(order, name)
		}
		if px.Online {
			now[name] = true
		}
	}

	var out []Event
	for _, name := range order {
		if !prevSeen[name] {
			continue // новая группа — не алертим на появление
		}
		was, is := prev[name], now[name]
		if was && !is {
			out = append(out, Event{Type: EventServerDown, Name: name, Online: false})
		} else if !was && is {
			out = append(out, Event{Type: EventServerUp, Name: name, Online: true})
		}
	}
	return out
}
