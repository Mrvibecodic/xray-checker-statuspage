package store

// bot_state — лёгкое состояние диалога бота (FSM для ввода значений кнопками).

func (s *Store) SetBotState(chatID int64, key, value string) error {
	_, err := s.exec(`INSERT INTO bot_state(chat_id,key,value) VALUES(?,?,?)
		ON CONFLICT(chat_id,key) DO UPDATE SET value=excluded.value`, chatID, key, value)
	return err
}

func (s *Store) GetBotState(chatID int64, key string) string {
	var v string
	if err := s.queryRow(`SELECT value FROM bot_state WHERE chat_id=? AND key=?`, chatID, key).Scan(&v); err != nil {
		return ""
	}
	return v
}

func (s *Store) DelBotState(chatID int64, key string) error {
	_, err := s.exec(`DELETE FROM bot_state WHERE chat_id=? AND key=?`, chatID, key)
	return err
}
