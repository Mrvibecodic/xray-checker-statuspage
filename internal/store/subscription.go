package store

// secretSubURL — ключ секрета с upstream-подпиской (содержит креды → шифруется).
const secretSubURL = "sub_url"

// SetSubscriptionURL сохраняет upstream-подписку (шифрованно).
func (s *Store) SetSubscriptionURL(url string) error { return s.SetSecret(secretSubURL, url) }

// SubscriptionURL возвращает сохранённую upstream-подписку.
func (s *Store) SubscriptionURL() (string, error) { return s.GetSecret(secretSubURL) }

// HasSubscriptionURL — настроена ли подписка.
func (s *Store) HasSubscriptionURL() bool { return s.HasSecret(secretSubURL) }

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
