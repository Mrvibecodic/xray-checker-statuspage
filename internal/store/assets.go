package store

// Бинарные ассеты, задаваемые из бота (например, фавикон). Хранятся в БД, чтобы
// переживать рестарт и работать в обоих драйверах.

func (s *Store) SetAsset(k, mime string, data []byte) error {
	_, err := s.exec(
		`INSERT INTO assets(k,mime,data) VALUES(?,?,?)
		 ON CONFLICT(k) DO UPDATE SET mime=excluded.mime, data=excluded.data`,
		k, mime, data)
	return err
}

func (s *Store) GetAsset(k string) (mime string, data []byte, ok bool) {
	if err := s.queryRow(`SELECT mime,data FROM assets WHERE k=?`, k).Scan(&mime, &data); err != nil {
		return "", nil, false
	}
	return mime, data, len(data) > 0
}

func (s *Store) DelAsset(k string) error {
	_, err := s.exec(`DELETE FROM assets WHERE k=?`, k)
	return err
}
