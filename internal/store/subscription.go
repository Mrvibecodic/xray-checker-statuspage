package store

import "strings"

// secretSubURL — ключ секрета со списком upstream-подписок (содержат креды →
// шифруется). Несколько подписок хранятся одной строкой через перенос строки.
const secretSubURL = "sub_url"

// SetSubscriptionURL сохраняет upstream-подписку (шифрованно). Перезаписывает
// весь список — используется командой /sub url и обратной совместимостью.
func (s *Store) SetSubscriptionURL(url string) error { return s.SetSecret(secretSubURL, url) }

// SubscriptionURL возвращает сохранённый секрет подписки как есть (одной строкой).
func (s *Store) SubscriptionURL() (string, error) { return s.GetSecret(secretSubURL) }

// SubscriptionURLs возвращает список сохранённых подписок (по строкам, без дублей).
func (s *Store) SubscriptionURLs() []string {
	raw, err := s.GetSecret(secretSubURL)
	if err != nil {
		return nil
	}
	return splitSubLines(raw)
}

// splitSubLines делит хранимый секрет на отдельные подписки (перенос строки —
// разделитель; URL не содержат сырых переносов).
func splitSubLines(raw string) []string {
	var out []string
	seen := map[string]bool{}
	for _, l := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		if l = strings.TrimSpace(l); l != "" && !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	return out
}

// AddSubscriptionURLs добавляет подписки к текущему списку БЕЗ перезаписи и без
// дублей. Возвращает число фактически добавленных.
func (s *Store) AddSubscriptionURLs(urls []string) (int, error) {
	cur := s.SubscriptionURLs()
	seen := map[string]bool{}
	for _, u := range cur {
		seen[u] = true
	}
	added := 0
	for _, u := range urls {
		if u = strings.TrimSpace(u); u == "" || seen[u] {
			continue
		}
		seen[u] = true
		cur = append(cur, u)
		added++
	}
	if added == 0 {
		return 0, nil
	}
	return added, s.SetSecret(secretSubURL, strings.Join(cur, "\n"))
}

// RemoveSubscriptionURLAt удаляет подписку по индексу (0-based). Возвращает
// удалённый URL ("" если индекс вне диапазона).
func (s *Store) RemoveSubscriptionURLAt(i int) (string, error) {
	cur := s.SubscriptionURLs()
	if i < 0 || i >= len(cur) {
		return "", nil
	}
	removed := cur[i]
	cur = append(cur[:i], cur[i+1:]...)
	return removed, s.SetSecret(secretSubURL, strings.Join(cur, "\n"))
}

// HasSubscriptionURL — задана ли хотя бы одна подписка.
func (s *Store) HasSubscriptionURL() bool { return len(s.SubscriptionURLs()) > 0 }

type ServerMeta struct {
	Name    string
	Enabled bool
}

// SetServerEnabled включает/выключает сервер в отдаваемой подписке.
func (s *Store) SetServerEnabled(name string, enabled bool) error {
	e := 0
	if enabled {
		e = 1
	}
	_, err := s.exec(`INSERT INTO servers_meta(name,enabled) VALUES(?,?)
		ON CONFLICT(name) DO UPDATE SET enabled=excluded.enabled`, name, e)
	return err
}

// DisabledServers — множество имён, выключенных из подписки.
func (s *Store) DisabledServers() (map[string]bool, error) {
	rows, err := s.query(`SELECT name FROM servers_meta WHERE enabled=0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out[n] = true
	}
	return out, rows.Err()
}

// ServersMeta — все записи servers_meta (для /sub-обзора в боте).
func (s *Store) ServersMeta() ([]ServerMeta, error) {
	rows, err := s.query(`SELECT name, enabled FROM servers_meta ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ServerMeta
	for rows.Next() {
		var m ServerMeta
		var e int
		if err := rows.Scan(&m.Name, &e); err != nil {
			return nil, err
		}
		m.Enabled = e == 1
		out = append(out, m)
	}
	return out, rows.Err()
}
