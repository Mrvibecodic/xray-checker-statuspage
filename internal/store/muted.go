package store

// MutedSet — множество имён серверов, по которым админ заглушил алерты.
func (s *Store) MutedSet() (map[string]bool, error) {
	rows, err := s.query(`SELECT name FROM muted`)
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

// SetMutedName включает/выключает заглушку алертов по серверу (по имени).
// Заглушённый сервер остаётся в мониторинге и на странице — молчат только уведы.
func (s *Store) SetMutedName(name string, mute bool) error {
	if mute {
		q := `INSERT OR IGNORE INTO muted(name) VALUES(?)`
		if s.d.isPG() {
			q = `INSERT INTO muted(name) VALUES(?) ON CONFLICT DO NOTHING`
		}
		_, err := s.exec(q, name)
		return err
	}
	_, err := s.exec(`DELETE FROM muted WHERE name=?`, name)
	return err
}
