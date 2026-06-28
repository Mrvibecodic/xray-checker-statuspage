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
	prev := map[string]bool{}     // ключ группы -> online
	prevSeen := map[string]bool{} // группа присутствовала
	for _, r := range prevRows {
		k := groupKey(r.Grp, r.Name)
		prevSeen[k] = true
		if r.Online == 1 {
			prev[k] = true
		}
	}

	// now: онлайн-состояние групп из свежего опроса. Балансир (непустой grp) —
	// одна группа на все свои узлы: «онлайн», если жив хотя бы один. Узлы
	// балансира имеют разные имена, поэтому ключ — именно grp, а не name.
	now := map[string]bool{}
	seen := map[string]bool{}
	order := []string{}
	dispOf := map[string]string{}
	for _, px := range proxies {
		if px.StableID == "" {
			continue
		}
		name := px.Name
		if name == "" {
			name = px.StableID
		}
		k := groupKey(px.GroupName, name)
		disp := name
		if px.GroupName != "" {
			disp = px.GroupName
		}
		if !seen[k] {
			seen[k] = true
			order = append(order, k)
			dispOf[k] = disp
		}
		if px.Online {
			now[k] = true
		}
	}

	var out []Event
	for _, k := range order {
		if !prevSeen[k] {
			continue // новая группа — не алертим на появление
		}
		was, is := prev[k], now[k]
		if was && !is {
			out = append(out, Event{Type: EventServerDown, Name: dispOf[k], Online: false})
		} else if !was && is {
			out = append(out, Event{Type: EventServerUp, Name: dispOf[k], Online: true})
		}
	}
	return out
}

// groupKey — ключ агрегации хоста для алертов: балансировочная группа (непустой
// grp) считается одним хостом; иначе — отдельный сервер по имени. Префиксы
// разводят пространства, чтобы grp и name никогда не столкнулись.
func groupKey(grp, name string) string {
	if grp != "" {
		return "g:" + grp
	}
	return "n:" + name
}
