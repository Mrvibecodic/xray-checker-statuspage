package store

// ConsolidateByName схлопывает историю серверов с ОДИНАКОВЫМ именем на один
// stableId. Нужно, когда у сервера сменился stableId, хотя сам сервер тот же —
// например, при обновлении чекера, сменившего схему генерации id. Тогда старый id
// перестаёт приходить в опросе, а его daily/samples «отвязываются» от плитки →
// пустая (белая) история.
//
// Переносим историю ОТСУТСТВУЮЩЕГО в последнем опросе id на актуальный
// одноимённый и удаляем его строку current.
//
// Защита от нескольких подписок и повторяющихся имён: переносится ТОЛЬКО id,
// которого не было в последнем опросе (ts < lastPollTS). Два одновременно
// активных одноимённых сервера (оба присутствуют в текущем опросе) НЕ
// объединяются — это разные сервера. Идемпотентно; безопасно звать каждый цикл.
func (s *Store) ConsolidateByName(lastPollTS int64) (int, error) {
	rows, err := s.query(`SELECT sid, name, ts FROM current`)
	if err != nil {
		return 0, err
	}
	type rec struct {
		sid string
		ts  int64
	}
	byName := map[string][]rec{}
	for rows.Next() {
		var sid, name string
		var ts int64
		if err := rows.Scan(&sid, &name, &ts); err != nil {
			rows.Close()
			return 0, err
		}
		byName[name] = append(byName[name], rec{sid, ts})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	merged := 0
	for _, list := range byName {
		if len(list) < 2 {
			continue
		}
		// Канонический id — с максимальным ts (актуальная личность сервера).
		canon := list[0]
		for _, r := range list[1:] {
			if r.ts > canon.ts {
				canon = r
			}
		}
		for _, r := range list {
			// Не трогаем сам канон и одновременно активные одноимённые сервера
			// (ts >= lastPollTS): это либо тот же канон, либо другой реальный
			// сервер из другой подписки с тем же именем.
			if r.sid == canon.sid || r.ts >= lastPollTS {
				continue
			}
			if err := s.mergeSid(r.sid, canon.sid); err != nil {
				return merged, err
			}
			merged++
		}
	}
	return merged, nil
}

// mergeSid переносит daily (суммируя по дню) и samples с from на to, затем
// удаляет from из current. Транзакционно.
func (s *Store) mergeSid(from, to string) (err error) {
	tx, err := s.begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.Exec(`INSERT INTO daily(day,sid,up,down,lat_sum,lat_cnt,down_conf)
		SELECT day,?,up,down,lat_sum,lat_cnt,down_conf FROM daily WHERE sid=?
		ON CONFLICT(day,sid) DO UPDATE SET
		  up=daily.up+excluded.up, down=daily.down+excluded.down,
		  lat_sum=daily.lat_sum+excluded.lat_sum, lat_cnt=daily.lat_cnt+excluded.lat_cnt,
		  down_conf=daily.down_conf+excluded.down_conf`, to, from); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM daily WHERE sid=?`, from); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE samples SET sid=? WHERE sid=?`, to, from); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM current WHERE sid=?`, from); err != nil {
		return err
	}
	return tx.Commit()
}
