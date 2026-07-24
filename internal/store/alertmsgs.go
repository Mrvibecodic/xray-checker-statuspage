package store

// AlertMsg — отправленный в чат алерт, который надо удалить в момент del_at
// (автоочистка алертов: настройка alert_ttl_hours).
type AlertMsg struct {
	ChatID int64
	MsgID  int
	DelAt  int64
}

// AddAlertMessage запоминает алерт для последующего автоудаления из чата.
func (s *Store) AddAlertMessage(chatID int64, msgID int, delAt int64) error {
	_, err := s.exec(`INSERT INTO alert_msgs(chat_id,msg_id,del_at) VALUES(?,?,?)`,
		chatID, msgID, delAt)
	return err
}

// DueAlertMessages — алерты, чей срок удаления уже наступил.
func (s *Store) DueAlertMessages(now int64) ([]AlertMsg, error) {
	rows, err := s.query(`SELECT chat_id,msg_id,del_at FROM alert_msgs WHERE del_at<=?`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AlertMsg
	for rows.Next() {
		var m AlertMsg
		if err := rows.Scan(&m.ChatID, &m.MsgID, &m.DelAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// RemoveAlertMessage убирает запись об алерте (после попытки удаления из чата —
// независимо от её исхода: сообщение старше 48ч или удалённое руками Telegram
// уже не отдаст, повторять бессмысленно).
func (s *Store) RemoveAlertMessage(chatID int64, msgID int) error {
	_, err := s.exec(`DELETE FROM alert_msgs WHERE chat_id=? AND msg_id=?`, chatID, msgID)
	return err
}
