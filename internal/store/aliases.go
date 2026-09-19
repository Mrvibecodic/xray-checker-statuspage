package store

import "strings"

// AliasMaxRunes — предел длины имени для страницы (плитка не резиновая).
const AliasMaxRunes = 64

// Aliases — имена серверов для публичной страницы, заданные в боте:
// исходное имя (как в опросе; для балансира — имя группы) -> имя на странице.
func (s *Store) Aliases() (map[string]string, error) {
	rows, err := s.query(`SELECT name, alias FROM aliases`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var n, a string
		if err := rows.Scan(&n, &a); err != nil {
			return nil, err
		}
		out[n] = a
	}
	return out, rows.Err()
}

// SetAlias задаёт имя сервера на странице; пустое — сброс к исходному.
func (s *Store) SetAlias(name, alias string) error {
	alias = strings.Join(strings.Fields(alias), " ") // переносы строк/табы — в пробел
	if r := []rune(alias); len(r) > AliasMaxRunes {
		alias = string(r[:AliasMaxRunes])
	}
	if alias == "" || alias == name {
		_, err := s.exec(`DELETE FROM aliases WHERE name=?`, name)
		return err
	}
	_, err := s.exec(`INSERT INTO aliases(name,alias) VALUES(?,?)
		ON CONFLICT(name) DO UPDATE SET alias=excluded.alias`, name, alias)
	return err
}
